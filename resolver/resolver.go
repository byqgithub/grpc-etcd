package resolver

import (
	"context"
	"fmt"
	"sync"
	"time"

	etcdClientV3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/api/v3/mvccpb"
	"google.golang.org/grpc/resolver"

	"grpc-etcd/utils"
)

type BuildResolver struct {
	// key of grpc resolver builder map 
	scheme       string

	// etcd
	etcdAddr         []string
	etcdDialTimeout  int64
	etcdKey          string
}

func NewResolverBuilder(scheme string, etcdAddr []string, dialTimeout int64, etcdKey string) {
	br := &BuildResolver{
		scheme: scheme,
		etcdAddr: etcdAddr,
		etcdDialTimeout: dialTimeout,
		etcdKey: etcdKey,
	}
	resolver.Register(br)
	fmt.Println("Create resolver builder")
}

func (br *BuildResolver) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	sr := newServiceResolver(br.etcdKey)
	err := sr.initSR(cc, br.etcdAddr, br.etcdDialTimeout)
	if err == nil {
		fmt.Println("Resolver-Builder create Service-Resolver")
	}
	return sr, err
}

func (br *BuildResolver) Scheme() string {
	fmt.Printf("Return resolver scheme: %s\n", br.scheme)
	return br.scheme
}

type ServiceResolver struct {
	ctx          context.Context
	cancel       context.CancelFunc
	// wg is used to enforce Close() to return after the watcher() goroutine has
	// finished. Otherwise, data race will be possible. [Race Example] in
	// dns_resolver_test we replace the real lookup functions with mocked ones to
	// facilitate testing. If Close() doesn't wait for watcher() goroutine
	// finishes, race detector sometimes will warns lookup (READ the lookup
	// function pointers) inside watcher() goroutine has data race with
	// replaceNetFunc (WRITE the lookup function pointers).
	wg           sync.WaitGroup

	// etcd
	etcdClient       *etcdClientV3.Client
	etcdWatchCh      etcdClientV3.WatchChan
	etcdKey          string

	// 服务地址列表
	serviceAddrList  []resolver.Address

	// grpc resolver
	cc           resolver.ClientConn

	// 在 ResolveNow() 使用，通知监控协程立刻更新服务器地址
	rChan        chan struct{}
}

func newServiceResolver(etcdKey string) *ServiceResolver {
	return &ServiceResolver{
		etcdKey: etcdKey,
		rChan: make(chan struct{}),
		serviceAddrList: make([]resolver.Address, 0),
	}
}

func (sr *ServiceResolver) initSR(cc resolver.ClientConn, addr []string, timeout int64) error {
	var err error
	sr.cc = cc

	// 创建 etcd 客户端
	sr.etcdClient, err = etcdClientV3.New(etcdClientV3.Config{
		Endpoints: addr,
		DialTimeout: time.Duration(timeout) * time.Second,
	})
	if err != nil {
		fmt.Printf("Init Service-Resolver Error: Resolver create etcd client error: %v\n", err)
		return err
	} else {
		fmt.Println("Service-Resolver create etcd client")
	}

	sr.ctx, sr.cancel = context.WithCancel(context.Background())
	sr.etcdWatchCh = sr.etcdClient.Watch(sr.ctx, sr.etcdKey, etcdClientV3.WithPrefix(), etcdClientV3.WithPrevKV())
	fmt.Println("Service-Resolver create etcd watch channel")

	_ = sr.sync()   // 第一次更新服务器地址
	sr.wg.Add(1)
	go sr.watcher() // 启动协程，持续更新服务器地址

	return nil
}

func (sr *ServiceResolver) watcher() {
	fmt.Println("Service-Resolver start watcher")
	ticker := time.NewTicker(time.Minute)
	defer sr.wg.Done()
	defer ticker.Stop()

	for {
		select {
		case <- ticker.C:  // 每分钟从 etcd 同步服务器地址
			sr.sync()
		case <- sr.ctx.Done():
			fmt.Println("Service-Resolver watcher stop")
			return
		case out, ok := <- sr.etcdWatchCh:  // 利用 etcd watch 机制更新服务器地址
			if ok {
				sr.update(out.Events)
			}
		}
	}
}

func (sr *ServiceResolver) update(events []*etcdClientV3.Event) {
	for _, event := range events {
		switch event.Type {
		case mvccpb.PUT:
			info, err := utils.ParseValue(event.Kv.Value)
			if err != nil {
				continue
			}
			addr := resolver.Address{Addr: info.Addr, Metadata: info.Weight}
			if !sr.addrExist(addr) {
				sr.serviceAddrList = append(sr.serviceAddrList, addr)
				sr.cc.UpdateState(resolver.State{Addresses: sr.serviceAddrList})
			}
		case mvccpb.DELETE:
			info, err := utils.ParseValue(event.Kv.Value)
			if err != nil {
				continue
			}
			addr := resolver.Address{Addr: info.Addr, Metadata: info.Weight}
			sr.delAddr(addr)
			sr.cc.UpdateState(resolver.State{Addresses: sr.serviceAddrList})
		default:
			fmt.Printf("Etcd event type: %v, unkown type\n", event.Type)
		}
	}
}

func (sr *ServiceResolver) sync() error {
	out, err := sr.etcdClient.Get(sr.ctx, sr.etcdKey, etcdClientV3.WithPrefix())
	if err != nil {
		fmt.Printf("Service-Resolver sync from etcd error: %v\n", err)
		return err
	}

	sr.serviceAddrList = make([]resolver.Address, 0)
	for _, value := range out.Kvs {
		info, err := utils.ParseValue(value.Value)
		if err != nil {
			fmt.Printf("Service-Resolver sync parse etcd value error: %v\n", err)
			continue
		}
		addr := resolver.Address{Addr: info.Addr, Metadata: info.Weight}
		sr.serviceAddrList = append(sr.serviceAddrList, addr)
	}

	sr.cc.UpdateState(resolver.State{Addresses: sr.serviceAddrList})
	return nil
}

func (sr *ServiceResolver) addrExist(addr resolver.Address) bool {
	for _, tmp := range sr.serviceAddrList {
		if addr.Addr == tmp.Addr {
			return true
		}
	}
	return false
}

func (sr *ServiceResolver) delAddr(addr resolver.Address) {
	for i, tmp := range sr.serviceAddrList {
		if tmp.Addr == addr.Addr {
			sr.serviceAddrList[i] = sr.serviceAddrList[len(sr.serviceAddrList)-1]
			sr.serviceAddrList = sr.serviceAddrList[:len(sr.serviceAddrList)-1]
		}
	}
}

func (sr *ServiceResolver) ResolveNow(resolver.ResolveNowOptions) {
	fmt.Println("Service-Resolver update now")
	sr.rChan <- struct{}{}
}

// Close closes the resolver.
func (sr *ServiceResolver) Close() {
	sr.cancel()
	sr.wg.Wait()
	sr.etcdClient.Close()
	close(sr.rChan)
	fmt.Println("Service-Resolver stop")
}