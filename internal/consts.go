package internal

const (
	Scheme = "etcd"
	Endpoint = "internal-app"
    ClientDialAddr = Scheme + "///" + Endpoint

	EtcdAddr = "127.0.0.1:2379"
	// EtcdKey = Endpoint

	Serviceip = "127.0.0.1"
	ServicePort = 15626
)