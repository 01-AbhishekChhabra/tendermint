package peer

import (
    . "github.com/tendermint/tendermint/common"
    "sync/atomic"
    "net"
)

const (
    DEFAULT_PORT = 8001
)

/* Listener */

type Listener interface {
    Connections()   <-chan *Connection
    LocalAddress()  *NetAddress
    Stop()
}


/* DefaultListener */

type DefaultListener struct {
    listener        net.Listener
    connections     chan *Connection
    stopped         uint32
}

const (
    DEFAULT_BUFFERED_CONNECTIONS = 10
)

func NewDefaultListener(protocol string, listenAddr string) Listener {
    listener, err := net.Listen(protocol, listenAddr)
    if err != nil { panic(err) }

    dl := &DefaultListener{
        listener:       listener,
        connections:    make(chan *Connection, DEFAULT_BUFFERED_CONNECTIONS),
    }

    go dl.listenHandler()

    return dl
}

func (l *DefaultListener) listenHandler() {
    for {
        conn, err := l.listener.Accept()

        if atomic.LoadUint32(&l.stopped) == 1 { return }

        // listener wasn't stopped,
        // yet we encountered an error.
        if err != nil { panic(err) }

        c := NewConnection(conn)
        l.connections <- c
    }

    // cleanup
    close(l.connections)
    for _ = range l.connections {
        // drain
    }
}

func (l *DefaultListener) Connections() <-chan *Connection {
    return l.connections
}

func (l *DefaultListener) LocalAddress() *NetAddress {
    return GetLocalAddress()
}

func (l *DefaultListener) Stop() {
    if atomic.CompareAndSwapUint32(&l.stopped, 0, 1) {
        l.listener.Close()
    }
}


/* local address helpers */

func GetLocalAddress() *NetAddress {
    laddr := GetUPNPLocalAddress()
    if laddr != nil { return laddr }

    laddr = GetDefaultLocalAddress()
    if laddr != nil { return laddr }

    panic("Could not determine local address")
}

// UPNP external address discovery & port mapping
// TODO: more flexible internal & external ports
func GetUPNPLocalAddress() *NetAddress {
    nat, err := Discover()
    if err != nil { return nil }

    ext, err := nat.GetExternalAddress()
    if err != nil { return nil }

    _, err = nat.AddPortMapping("tcp", DEFAULT_PORT, DEFAULT_PORT, "tendermint", 0)
    if err != nil { return nil }

    return NewNetAddressIPPort(ext, DEFAULT_PORT)
}

// Naive local IPv4 interface address detection
// TODO: use syscalls to get actual ourIP. http://pastebin.com/9exZG4rh
func GetDefaultLocalAddress() *NetAddress {
    addrs, err := net.InterfaceAddrs()
    if err != nil { Panicf("Unexpected error fetching interface addresses: %v", err) }

    for _, a := range addrs {
        ipnet, ok := a.(*net.IPNet)
        if !ok { continue }
        v4 := ipnet.IP.To4()
        if v4 == nil || v4[0] == 127 { continue } // loopback
        return NewNetAddressIPPort(ipnet.IP, DEFAULT_PORT)
    }
    return nil
}


