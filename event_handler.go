package gain

type EventHandler interface {
	OnOpen(fd int)
	OnClose(fd int)
	OnData(c Conn) error
	AfterWrite(c Conn)
}

type DefaultEventHandler struct{}

func (e DefaultEventHandler) OnOpen(fd int)     {}
func (e DefaultEventHandler) OnClose(fd int)    {}
func (e DefaultEventHandler) OnData(c Conn)     {}
func (e DefaultEventHandler) AfterWrite(c Conn) {}
