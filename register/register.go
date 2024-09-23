package register

import (
	"context"
	"fmt"
	"sync"
	"time"

	etcdClientV3 "go.etcd.io/etcd/client/v3"

	"grpc-etcd/utils"
)

type Register struct {
	// etcd client params
	etcdAddr         []string
	etcdDialTimeout  int64

	etcdClient       *etcdClientV3.Client
	leasesIDMap      map[string]etcdClientV3.LeaseID
	keepAliveChMap   map[string]<-chan *etcdClientV3.LeaseKeepAliveResponse

	// service info
	serviceInfoList  []utils.ServiceInfo

	// 停止 register 协程
	ctx              context.Context
	cancel           context.CancelFunc 
	wg               sync.WaitGroup
}

func NewRegister(addr []string, dialTimeout int64) *Register {
	r :=  &Register{
		etcdAddr: addr,
		etcdDialTimeout: dialTimeout,
	}
	r.leasesIDMap = make(map[string]etcdClientV3.LeaseID, 0)
	r.keepAliveChMap = make(map[string]<-chan *etcdClientV3.LeaseKeepAliveResponse, 0)
	r.serviceInfoList = make([]utils.ServiceInfo, 0)

	_ = r.initRegister()
	r.ctx, r.cancel = context.WithCancel(context.Background())

	return r
}

func (r *Register) initRegister() error {
	var err error

	// 创建 etcd 客户端
	r.etcdClient, err = etcdClientV3.New(etcdClientV3.Config{
		Endpoints: r.etcdAddr,
		DialTimeout: time.Duration(r.etcdDialTimeout) * time.Second,
	})
	if err != nil {
		fmt.Printf("Init Service-Register Error: Register create etcd client error: %v\n", err)
		return err
	} else {
		fmt.Println("Service-Register create etcd client")
	}

	r.wg.Add(1)
	go r.keepAlive()

	return nil
}

func (r *Register) ServiceRegister(info utils.ServiceInfo) error {
	r.serviceInfoList = append(r.serviceInfoList, info)
	key := info.ServerEtecKey()
	value, err := info.Encoder()
	if err != nil {
		return err
	}

	// etcd grant lease
	grantCtx, grantCancel := context.WithTimeout(r.ctx, time.Duration(r.etcdDialTimeout) * time.Second)
	defer grantCancel()
	response, err := r.etcdClient.Grant(grantCtx, info.TTL)
	if err != nil {
		fmt.Printf("Service-Register grant etcd lease error: %v \n", err)
		return err
	}
	r.leasesIDMap[key] = response.ID

	// etcd keepAlive
	tmpCh, err := r.etcdClient.KeepAlive(r.ctx, response.ID)
	if err != nil {
		fmt.Printf("Service-Register etcd keepAlive key-value error: %v\n", err)
		return err
	}
	r.keepAliveChMap[key] = tmpCh

	// etcd put key-value
	putCtx, putCancel := context.WithCancel(r.ctx)
	defer putCancel()
	_, err = r.etcdClient.Put(putCtx, key, value, etcdClientV3.WithLease(response.ID))
	if err != nil {
		fmt.Printf("Service-Register etcd put data error: %v\n", err)
		return err
	}

	return nil
}

func (r *Register) Unregister(info utils.ServiceInfo) error {
	key := info.ServerEtecKey()
	// 使用删除租期的方式，删除与租期关联的键值
	if leaseID, ok := r.leasesIDMap[key]; ok {
		_, err := r.etcdClient.Revoke(context.Background(), leaseID)
		if err == nil {
			fmt.Printf("Service-Register etcd revoke key(%s) lease successfully\n", key)
			return nil
		}

		fmt.Printf("Service-Register etcd revoke lease error: %v\n", err)
	}

	// 删除租期异常时，尝试直接删除键值
	_, err := r.etcdClient.Delete(context.Background(), key)
	if err != nil {
		fmt.Printf("Service-Register etcd delete key(%s) error: %v\n", key, err)
		return err
	}

	return nil
}

func (r *Register) keepAlive() {
	defer r.wg.Done()

	ticker := time.NewTicker(time.Duration(r.etcdDialTimeout) * time.Second)
	defer ticker.Stop()

	select {
	case <- ticker.C:
		
		for _, info := range r.serviceInfoList {
			// 检测是否还有没有自动续约租期的服务
			if ch, ok := r.keepAliveChMap[info.ServerEtecKey()]; !ok {
				if err := r.ServiceRegister(info); err != nil {
					fmt.Printf("Service-Register retry to register %v error: %v\n", info.ServerEtecKey(), err)
				}
			} else {
				// 检测是否有续约异常服务
				select {
				case out := <-ch:
					if out == nil {
						if err := r.ServiceRegister(info); err != nil {
							fmt.Printf("Service-Register: %v retry to grant lease error: %v\n", info.ServerEtecKey(), err)
						}
					}
				default:
				}
			}
		}
	case <- r.ctx.Done():
		for _, info := range r.serviceInfoList {
			err := r.Unregister(info)
			if err != nil {
				fmt.Printf("Service-Register unregister %v service error: %v\n", info.ServerEtecKey(), err)
			} else {
				fmt.Printf("Service-Register unregister %v service successfully\n", info.ServerEtecKey())
			}
		}
		fmt.Println("Service-Register keepAlive goroutine stop")
		return
		
	}
}

func (r *Register) Stop() {
	r.cancel()
	r.wg.Wait()
	r.etcdClient.Close()
	fmt.Println("Service-Register Stop")
}