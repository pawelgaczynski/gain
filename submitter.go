package gain

type submitter interface {
	submit() error
	advance(n uint32)
}
