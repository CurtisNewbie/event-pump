package pump

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/curtisnewbie/miso/miso"
	"github.com/curtisnewbie/miso/util"
	"github.com/go-zookeeper/zk"
)

var (
	zkOnce    sync.Once
	zkc       *zk.Conn
	zkElectMu sync.Mutex
	zkWatched bool
)

const (
	PropHost     = "ha.zookeeper.host"
	ZkPathRoot   = "/eventpump"
	ZkPathLeader = ZkPathRoot + "/leader"
	ZkPathPos    = ZkPathRoot + "/pos"
)

func ConnZk() *zk.Conn {
	zkOnce.Do(func() {
		hosts := miso.GetPropStrSlice(PropHost)
		miso.Infof("Connecting to Zookeeper: %+v", hosts)
		c, _, err := zk.Connect(hosts, time.Second*5, func(zc *zk.Conn) {
			zc.SetLogger(miso.EmptyRail())
		})
		if err != nil {
			panic(fmt.Errorf("connect zookeeper failed, %v", err))
		}
		zkc = c
		miso.AddShutdownHook(func() { zkc.Close() })
	})
	if zkc == nil {
		panic(errors.New("missing zookeeper connection"))
	}

	return zkc
}

func ZkCreateEph(p string, dat []byte) error {
	_, err := ConnZk().Create(p, dat, zk.FlagEphemeral, zk.WorldACL(zk.PermAll))
	return err
}

func ZkCreatePer(p string, dat []byte) error {
	_, err := ConnZk().Create(p, dat, zk.FlagPersistent, zk.WorldACL(zk.PermAll))
	return err
}

func ZkWatch(p string) (<-chan zk.Event, error) {
	_, _, ch, err := ConnZk().ExistsW(p)
	return ch, err
}

func ZkGet(p string) ([]byte, error) {
	buf, _, err := ConnZk().Get(p)
	return buf, err
}

func ZkWritePos(buf []byte) error {
	_, err := ConnZk().Set(ZkPathPos, buf, -1)
	if err != nil && errors.Is(err, zk.ErrNoNode) {
		return ZkCreatePer(ZkPathPos, buf)
	}
	return err
}

func ZkReadPos() ([]byte, error) {
	buf, err := ZkGet(ZkPathPos)
	if err != nil && errors.Is(err, zk.ErrNoNode) {
		return nil, nil
	}
	return buf, err
}

func ZkElectLeader(rail miso.Rail, hook func()) error {
	zkElectMu.Lock()
	defer zkElectMu.Unlock()

	rootp := ZkPathRoot
	ip := miso.GetLocalIPV4()
	if err := ZkCreatePer(rootp, nil); err != nil {
		rail.Infof("Create parent path failed (expected), %v", err)
	}

	leaderp := ZkPathLeader
	if !zkWatched {
		rail.Infof("Watching zk path: %v", leaderp)
		ch, err := ZkWatch(leaderp)
		if err != nil {
			return err
		}
		zkWatched = true
		go func() {
			rail, cancel := rail.NextSpan().WithCancel()
			miso.AddShutdownHook(func() { cancel() })
			c := rail.Context()
			for {
				select {
				case e := <-ch:
					if e.Path == leaderp && e.Type == zk.EventNodeDeleted {
						rail.Infof("received zknode event, %#v", e)
						if err := ZkElectLeader(rail, hook); err != nil {
							rail.Errorf("received EventNodeDeleted, failed to elect leader, %v", err)
						}
					}
				case <-c.Done():
					rail.Info("Exiting ElectLeader node watcher")
					return
				}
			}
		}()
	}

	err := ZkCreateEph(leaderp, util.UnsafeStr2Byt(ip))
	if err != nil {
		if errors.Is(err, zk.ErrNodeExists) {
			rail.Info("Another event-pump instance is already running, waiting to become leader")
			return nil
		}
		if errors.Is(err, zk.ErrConnectionClosed) {
			return err
		}
	} else {
		rail.Info("Elected to be the leader, running hook")
		hook() // becomes the leader
	}
	return nil
}
