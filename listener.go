package main

type listener struct {
	p      *program
	socket *rawSocket

	listenDone chan struct{}
}

func newListener(p *program) error {
	socket, err := newRawSocket(p.intf)
	if err != nil {
		return err
	}

	ls := &listener{
		p:          p,
		socket:     socket,
		listenDone: make(chan struct{}),
	}

	p.ls = ls
	return nil
}

func (ls *listener) run() {
	for {
		raw, err := ls.socket.Read()
		if err != nil {
			panic(err)
		}

		ls.p.ma.listen <- raw
		ls.p.mm.listen <- raw
		ls.p.mn.listen <- raw

		// join before reading again
		for i := 0; i < 3; i++ {
			<-ls.listenDone
		}
	}
}
