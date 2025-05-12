package main

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/hazelcast/hazelcast-go-client"
)

func distributedMapsDataLoss(config hazelcast.Config) {
  ctx := context.TODO()
  client, _ := hazelcast.StartNewClientWithConfig(ctx, config)
	defer client.Shutdown(ctx)

  mp, _ := client.GetMap(ctx, "Distributed-map-unexpected-kill")

	for i := 0; i < 1000; i++ {
    key := strconv.Itoa(i)
    value := fmt.Sprintf("value-%d", i)
    mp.Set(ctx, key, value)
    time.Sleep(10 * time.Millisecond)
    if i % 100 == 0 {
      print(i / 100, " sec\n")
    }
	}
}

func counterNoBlocking(wg *sync.WaitGroup, config hazelcast.Config) {
  defer wg.Done()

  ctx := context.TODO()
  client, _ := hazelcast.StartNewClientWithConfig(ctx, config)
  mp, _ := client.GetMap(ctx, "Distributed-Map-No-Blocking")
  mp.PutIfAbsent(ctx, "key", 0)
  for i := 0; i < 10000; i++ {
    val, _ := mp.Get(ctx, "key")
    mp.Set(ctx, "key", val.(int64) + 1)
  }
}

func counterPessimistickBlocking(wg *sync.WaitGroup, config hazelcast.Config) {
	defer wg.Done()

	ctx := context.TODO()
  client, _ := hazelcast.StartNewClientWithConfig(ctx, config)
  mp, _ := client.GetMap(ctx, "Distributed-Map-Pessimistic-Blocking")
  mp.PutIfAbsent(ctx, "key", 0)

	lockCtx := mp.NewLockContext(ctx)
	for i := 1; i <= 10000; i++ {
		mp.Lock(lockCtx, "key")
		val, _ := mp.Get(lockCtx, "key")
		mp.Set(lockCtx, "key", val.(int64) + 1)
		mp.Unlock(lockCtx, "key")
	}
}

func counterOptimisticBlocking(wg *sync.WaitGroup, config hazelcast.Config) {
	defer wg.Done()

  ctx := context.TODO()
  client, _ := hazelcast.StartNewClientWithConfig(ctx, config)
  mp, _ := client.GetMap(ctx, "Distributed-Map-Optimistic-Blocking")
  mp.PutIfAbsent(ctx, "key", 0)

	for i := 1; i <= 10000; i++ {
		for {
			val, _ := mp.Get(ctx, "key")
			if ok, _ := mp.ReplaceIfSame(ctx, "key",val.(int64), val.(int64) + 1); ok {
				break
			}
		}
	}
}

func run(counter func(wg *sync.WaitGroup, config hazelcast.Config), config hazelcast.Config) {
  start := time.Now()

  var wg sync.WaitGroup
  for i := 1; i <= 3; i++ {
    wg.Add(1)
    go counter(&wg, config)
  }
  wg.Wait()

  fmt.Println("Execution time: ", time.Since(start))
}

func queueProducer(wg *sync.WaitGroup, config hazelcast.Config) {
  ctx := context.TODO()
  client, _ := hazelcast.StartNewClientWithConfig(ctx, config)
  queue, _ := client.GetQueue(ctx, "queue")
  for i := 1; i < 100; i++ {
		queue.Put(ctx, int64(i))
    fmt.Println("Producing: ", int64(i))
	}
  queue.Put(ctx, int64(-1))
  wg.Done()
}

func queueConsumer(wg *sync.WaitGroup, config hazelcast.Config, name string) {
  ctx := context.TODO()
  client, _ := hazelcast.StartNewClientWithConfig(ctx, config)
  queue, _ := client.GetQueue(ctx, "queue")
  for i := 1; i < 100; i++ {
		val, _ := queue.Take(ctx)
		fmt.Println(name, " consuming: ", val.(int64))
    if val == int64(-1) {
      err := queue.Put(ctx, int64(-1))
      if err != nil {
        panic(fmt.Errorf("putting item to queue %w", err))
      }
      wg.Done()
    }
	}
}

func runQueueTest (config hazelcast.Config) {
  var wg sync.WaitGroup
  wg.Add(3)

  go queueProducer(&wg,config)
  time.Sleep(time.Second)

  go queueConsumer(&wg, config, "First")
  go queueConsumer(&wg, config, "Second")
  wg.Wait()
}

func main() {
  config := hazelcast.Config{}

  config.Cluster.Name = "lab2"
	config.Cluster.Network.SetAddresses("127.0.0.1:5701", "127.0.0.1:5702", "127.0.0.1:5703")

  // distributedMapsDataLoss(config)

	// run(counterNoBlocking, config)
	// run(counterPessimistickBlocking, config)
	// run(counterOptimisticBlocking, config)
  runQueueTest(config)

}
