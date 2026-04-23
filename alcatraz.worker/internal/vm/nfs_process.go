package vm

type NFSProcess interface {
	GetProcess() interface{}
	Kill() error
	Wait() error
}