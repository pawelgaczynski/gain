package gain

import (
	"github.com/pawelgaczynski/gain/iouring"
	"github.com/pawelgaczynski/gain/logger"
)

type acceptorWorkerConfig struct {
	workerConfig
}

type acceptorWorker struct {
	*acceptor
	*workerImpl
	config        acceptorWorkerConfig
	ring          *iouring.Ring
	loadBalancer  loadBalancer
	eventHandler  EventHandler
	addConnMethod func(consumer, int32) error
}

func (a *acceptorWorker) addConnViaRing(worker consumer, fileDescriptor int32) error {
	entry, err := a.ring.GetSQE()
	if err != nil {
		return err
	}
	entry.PrepareMsgRing(worker.ringFd(), uint32(fileDescriptor), addConnFlag, 0)
	entry.UserData = addConnFlag
	return nil
}

func (a *acceptorWorker) addConnViaQueue(worker consumer, fileDescriptor int32) error {
	return worker.addConnToQueue(int(fileDescriptor))
}

func (a *acceptorWorker) registerConsumer(consumer consumer) {
	a.loadBalancer.register(consumer)
}

func (a *acceptorWorker) closeRingAndConsumers() {
	err := a.syscallShutdownSocket(a.socket)
	if err != nil {
		a.logError(err).Int("socket", a.socket).Msg("Socket shutdown error")
	}
	err = a.loadBalancer.forEach(func(w consumer) error {
		w.shutdown()
		return nil
	})
	if err != nil {
		a.logError(err).Msg("Closing consumers error")
	}
}

func (a *acceptorWorker) loop(socket int) error {
	a.logInfo().Int("socket", socket).Msg("Starting acceptor loop...")
	a.socket = socket
	a.prepareHandler = func() error {
		_, err := a.addAcceptRequest()
		return err
	}
	a.shutdownHandler = func() bool {
		if a.needToShutdown() {
			a.onCloseHandler()
			a.markShutdownInProgress()
			return false
		}
		return true
	}
	err := a.startLoop(0, func(cqe *iouring.CompletionQueueEvent) error {
		_, err := a.addAcceptRequest()
		if err != nil {
			a.logError(err).
				Int("socket", socket).
				Msg("Add accept() request error")
			return err
		}

		if exit := a.processEvent(cqe); exit {
			return nil
		}

		if cqe.UserData()&addConnFlag > 0 {
			return nil
		}
		nextConsumer := a.loadBalancer.next()
		err = a.addConnMethod(nextConsumer, cqe.Res())
		if err != nil {
			a.logError(err).
				Int("socket", socket).
				Int32("conn fd", cqe.Res()).
				Msg("Add connection to consumer error")

			err = a.syscallShutdownSocket(int(cqe.Res()))
			if err != nil {
				a.logError(err).Int32("conn fd", cqe.Res()).Msg("Can't shutdown socket")
			}
			return nil
		}
		return nil
	})
	a.notifyFinish()
	return err
}

func newAcceptorWorker(
	config acceptorWorkerConfig, loadBalancer loadBalancer, eventHandler EventHandler, supportedFeatures SupportedFeatures,
) (*acceptorWorker, error) {
	ring, err := iouring.CreateRing()
	if err != nil {
		return nil, err
	}
	logger := logger.NewLogger("acceptor", config.loggerLevel, config.prettyLogger)
	connectionPool := newConnectionPool(
		1,
		config.bufferSize,
		config.prefillConnectionPool,
	)
	workerImpl, err := newWorkerImpl(ring, config.workerConfig, 0, logger)
	if err != nil {
		return nil, err
	}
	acceptor := &acceptorWorker{
		workerImpl:   workerImpl,
		acceptor:     newAcceptor(ring, connectionPool),
		config:       config,
		ring:         ring,
		eventHandler: eventHandler,
		loadBalancer: loadBalancer,
	}
	if supportedFeatures.ringsMessaging {
		acceptor.addConnMethod = acceptor.addConnViaRing
	} else {
		acceptor.addConnMethod = acceptor.addConnViaQueue
	}
	acceptor.onCloseHandler = acceptor.closeRingAndConsumers
	return acceptor, nil
}
