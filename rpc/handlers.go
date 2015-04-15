package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/tendermint/tendermint/binary"
	"github.com/tendermint/tendermint/events"
	"io/ioutil"
	"net/http"
	"reflect"
	"time"
)

func RegisterRPCFuncs(mux *http.ServeMux, funcMap map[string]*RPCFunc) {
	// HTTP endpoints
	for funcName, rpcFunc := range funcMap {
		mux.HandleFunc("/"+funcName, makeHTTPHandler(rpcFunc))
	}

	// JSONRPC endpoints
	mux.HandleFunc("/", makeJSONRPCHandler(funcMap))
}

func RegisterEventsHandler(mux *http.ServeMux, evsw *events.EventSwitch) {
	// websocket endpoint
	w := NewWebsocketManager(evsw)
	mux.HandleFunc("/events", w.websocketHandler) // 	websocket.Handler(w.eventsHandler))
}

//-------------------------------------
// function introspection

// holds all type information for each function
type RPCFunc struct {
	f        reflect.Value  // underlying rpc function
	args     []reflect.Type // type of each function arg
	returns  []reflect.Type // type of each return arg
	argNames []string       // name of each argument
}

// wraps a function for quicker introspection
func NewRPCFunc(f interface{}, args []string) *RPCFunc {
	return &RPCFunc{
		f:        reflect.ValueOf(f),
		args:     funcArgTypes(f),
		returns:  funcReturnTypes(f),
		argNames: args,
	}
}

// return a function's argument types
func funcArgTypes(f interface{}) []reflect.Type {
	t := reflect.TypeOf(f)
	n := t.NumIn()
	types := make([]reflect.Type, n)
	for i := 0; i < n; i++ {
		types[i] = t.In(i)
	}
	return types
}

// return a function's return types
func funcReturnTypes(f interface{}) []reflect.Type {
	t := reflect.TypeOf(f)
	n := t.NumOut()
	types := make([]reflect.Type, n)
	for i := 0; i < n; i++ {
		types[i] = t.Out(i)
	}
	return types
}

// function introspection
//-----------------------------------------------------------------------------
// rpc.json

// jsonrpc calls grab the given method's function info and runs reflect.Call
func makeJSONRPCHandler(funcMap map[string]*RPCFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) > 1 {
			WriteRPCResponse(w, NewRPCResponse(nil, fmt.Sprintf("Invalid JSONRPC endpoint %s", r.URL.Path)))
			return
		}
		b, _ := ioutil.ReadAll(r.Body)
		var request RPCRequest
		err := json.Unmarshal(b, &request)
		if err != nil {
			WriteRPCResponse(w, NewRPCResponse(nil, err.Error()))
			return
		}
		rpcFunc := funcMap[request.Method]
		if rpcFunc == nil {
			WriteRPCResponse(w, NewRPCResponse(nil, "RPC method unknown: "+request.Method))
			return
		}
		args, err := jsonParamsToArgs(rpcFunc, request.Params)
		if err != nil {
			WriteRPCResponse(w, NewRPCResponse(nil, err.Error()))
			return
		}
		returns := rpcFunc.f.Call(args)
		response, err := unreflectResponse(returns)
		if err != nil {
			WriteRPCResponse(w, NewRPCResponse(nil, err.Error()))
			return
		}
		WriteRPCResponse(w, NewRPCResponse(response, ""))
	}
}

// covert a list of interfaces to properly typed values
func jsonParamsToArgs(rpcFunc *RPCFunc, params []interface{}) ([]reflect.Value, error) {
	values := make([]reflect.Value, len(params))
	for i, p := range params {
		ty := rpcFunc.args[i]
		v, err := _jsonObjectToArg(ty, p)
		if err != nil {
			return nil, err
		}
		values[i] = v
	}
	return values, nil
}

func _jsonObjectToArg(ty reflect.Type, object interface{}) (reflect.Value, error) {
	var err error
	v := reflect.New(ty)
	binary.ReadJSONFromObject(v.Interface(), object, &err)
	if err != nil {
		return v, err
	}
	v = v.Elem()
	return v, nil
}

// rpc.json
//-----------------------------------------------------------------------------
// rpc.http

// convert from a function name to the http handler
func makeHTTPHandler(rpcFunc *RPCFunc) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		args, err := httpParamsToArgs(rpcFunc, r)
		if err != nil {
			WriteRPCResponse(w, NewRPCResponse(nil, err.Error()))
			return
		}
		returns := rpcFunc.f.Call(args)
		response, err := unreflectResponse(returns)
		if err != nil {
			WriteRPCResponse(w, NewRPCResponse(nil, err.Error()))
			return
		}
		WriteRPCResponse(w, NewRPCResponse(response, ""))
	}
}

// Covert an http query to a list of properly typed values.
// To be properly decoded the arg must be a concrete type from tendermint (if its an interface).
func httpParamsToArgs(rpcFunc *RPCFunc, r *http.Request) ([]reflect.Value, error) {
	argTypes := rpcFunc.args
	argNames := rpcFunc.argNames

	var err error
	values := make([]reflect.Value, len(argNames))
	for i, name := range argNames {
		ty := argTypes[i]
		arg := GetParam(r, name)
		values[i], err = _jsonStringToArg(ty, arg)
		if err != nil {
			return nil, err
		}
	}
	return values, nil
}

func _jsonStringToArg(ty reflect.Type, arg string) (reflect.Value, error) {
	var err error
	v := reflect.New(ty)
	binary.ReadJSON(v.Interface(), []byte(arg), &err)
	if err != nil {
		return v, err
	}
	v = v.Elem()
	return v, nil
}

// rpc.http
//-----------------------------------------------------------------------------
// rpc.websocket

const (
	WSConnectionReaperSeconds = 5
	MaxFailedSendsSeconds     = 10
	WriteChanBufferSize       = 10
)

// for requests coming in
type WSRequest struct {
	Type  string // subscribe or unsubscribe
	Event string
}

// for responses going out
type WSResponse struct {
	Event string
	Data  interface{}
	Error string
}

// a single websocket connection
// contains the listeners id
type Connection struct {
	id          string
	wsCon       *websocket.Conn
	writeChan   chan WSResponse
	quitChan    chan struct{}
	failedSends uint
}

// new websocket connection wrapper
func NewConnection(con *websocket.Conn) *Connection {
	return &Connection{
		id:        con.RemoteAddr().String(),
		wsCon:     con,
		writeChan: make(chan WSResponse, WriteChanBufferSize), // buffered. we keep track when its full
	}
}

// close the connection
func (c *Connection) Close() {
	c.wsCon.Close()
	close(c.writeChan)
	close(c.quitChan)
}

// main manager for all websocket connections
// holds the event switch
type WebsocketManager struct {
	websocket.Upgrader
	ew   *events.EventSwitch
	cons map[string]*Connection
}

func NewWebsocketManager(ew *events.EventSwitch) *WebsocketManager {
	return &WebsocketManager{
		ew:   ew,
		cons: make(map[string]*Connection),
		Upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// TODO
				return true
			},
		},
	}
}

func (wm *WebsocketManager) websocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := wm.Upgrade(w, r, nil)
	if err != nil {
		// TODO
		log.Error("Failed to upgrade to websocket connection", "error", err)
		return
	}
	wm.handleWebsocket(conn)

}

func (w *WebsocketManager) handleWebsocket(con *websocket.Conn) {
	// register connection
	c := NewConnection(con)
	w.cons[c.id] = c
	log.Info("New websocket connection", "origin", c.id)

	// read subscriptions/unsubscriptions to events
	go w.read(c)
	// write responses
	w.write(c)
}

// read from the socket and subscribe to or unsubscribe from events
func (w *WebsocketManager) read(con *Connection) {
	reaper := time.Tick(time.Second * WSConnectionReaperSeconds)
	for {
		select {
		case <-reaper:
			if con.failedSends > MaxFailedSendsSeconds {
				// sending has failed too many times.
				// kill the connection
				con.quitChan <- struct{}{}
			}
		default:
			var in []byte
			_, in, err := con.wsCon.ReadMessage()
			if err != nil {
				// an error reading the connection,
				// so kill the connection
				con.quitChan <- struct{}{}
			}
			var req WSRequest
			err = json.Unmarshal(in, &req)
			if err != nil {
				errStr := fmt.Sprintf("Error unmarshaling data: %s", err.Error())
				con.writeChan <- WSResponse{Error: errStr}
			}
			switch req.Type {
			case "subscribe":
				log.Info("New event subscription", "con id", con.id, "event", req.Event)
				w.ew.AddListenerForEvent(con.id, req.Event, func(msg interface{}) {
					resp := WSResponse{
						Event: req.Event,
						Data:  msg,
					}
					select {
					case con.writeChan <- resp:
						// yay
						con.failedSends = 0
					default:
						// channel is full
						// if this happens too many times,
						// close connection
						con.failedSends += 1
					}
				})
			case "unsubscribe":
				if req.Event != "" {
					w.ew.RemoveListenerForEvent(req.Event, con.id)
				} else {
					w.ew.RemoveListener(con.id)
				}
			default:
				con.writeChan <- WSResponse{Error: "Unknown request type: " + req.Type}
			}

		}
	}
}

// receives on a write channel and writes out to the socket
func (w *WebsocketManager) write(con *Connection) {
	n, err := new(int64), new(error)
	for {
		select {
		case msg := <-con.writeChan:
			buf := new(bytes.Buffer)
			binary.WriteJSON(msg, buf, n, err)
			if *err != nil {
				log.Error("Failed to write JSON WSResponse", "error", err)
			} else {
				//websocket.Message.Send(con.wsCon, buf.Bytes())
				if err := con.wsCon.WriteMessage(websocket.TextMessage, buf.Bytes()); err != nil {
					log.Error("Failed to write response on websocket", "error", err)
				}
			}
		case <-con.quitChan:
			w.closeConn(con)
			return
		}
	}
}

// close a connection and delete from manager
func (w *WebsocketManager) closeConn(con *Connection) {
	con.Close()
	delete(w.cons, con.id)
}

// rpc.websocket
//-----------------------------------------------------------------------------

// returns is Response struct and error. If error is not nil, return it
func unreflectResponse(returns []reflect.Value) (interface{}, error) {
	errV := returns[1]
	if errV.Interface() != nil {
		return nil, fmt.Errorf("%v", errV.Interface())
	}
	return returns[0].Interface(), nil
}
