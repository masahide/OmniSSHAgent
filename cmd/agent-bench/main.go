package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/kelseyhightower/envconfig"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type Specification struct {
	PERSISTENT  bool `default:"false"`
	CONCURRENCY int  `default:"10"`
	RUN_COUNT   int  `default:"100"`
}

type sshAgent interface {
	agent.Agent
	Close() error
}
type Agent struct {
	agent.ExtendedAgent
	net.Conn
}

type exAgent struct {
	agent.Agent
}

func (e *exAgent) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	return nil, nil
}

func (e *exAgent) Extension(string, []byte) ([]byte, error) {
	return nil, nil
}

func getKey() *agent.Key {
	var key *agent.Key

	a, err := newAgent()
	if err != nil {
		log.Fatal(err)
	}
	keys, err := a.List()
	if err != nil {
		log.Fatalf("Failed to list keys: %v", err)
	}
	if len(keys) == 0 {
		log.Fatalf("No keys found in SSH agent")
	}
	key = keys[0]
	a.Close()
	return key
}

func main() {
	s := Specification{}
	err := envconfig.Process("", &s)
	if err != nil {
		log.Fatal(err)
	}
	flag.BoolVar(&s.PERSISTENT, "persistent", s.PERSISTENT, "persistent mode")
	flag.IntVar(&s.CONCURRENCY, "c", s.CONCURRENCY, "Number of concurrency processing")
	flag.IntVar(&s.RUN_COUNT, "n", s.RUN_COUNT, "run count")
	flag.Parse()
	taskCh := make(chan struct{})
	doneCh := make(chan []time.Duration, s.CONCURRENCY)

	var wg sync.WaitGroup
	key := getKey()
	fmt.Printf("The key used for measurement:%s\n", key.String())
	fmt.Printf("Start %d worker\n", s.CONCURRENCY)
	for i := 0; i < s.CONCURRENCY; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker(key, taskCh, doneCh, s.PERSISTENT)
		}()
	}

	start := time.Now()
	go func() {
		for i := 0; i < s.RUN_COUNT; i++ {
			taskCh <- struct{}{}
		}
		close(taskCh)
	}()

	go func() {
		wg.Wait()
		close(doneCh)
	}()
	var allExecutionTimes []time.Duration
	for times := range doneCh {
		allExecutionTimes = append(allExecutionTimes, times...)
	}
	fmt.Printf("\ndone.\n")
	totalTime := time.Duration(0)
	var minTime, maxTime time.Duration
	minTime = allExecutionTimes[0]
	maxTime = allExecutionTimes[0]

	for _, t := range allExecutionTimes {
		totalTime += t
		if t < minTime {
			minTime = t
		}
		if t > maxTime {
			maxTime = t
		}
	}

	averageTime := totalTime / time.Duration(len(allExecutionTimes))

	sort.Slice(allExecutionTimes, func(i, j int) bool {
		return allExecutionTimes[i] < allExecutionTimes[j]
	})
	p99Time := allExecutionTimes[int(float64(len(allExecutionTimes))*0.99)-1]

	fmt.Printf("Real Time: %v\n", time.Since(start))
	fmt.Printf("Total Executions: %d\n", s.RUN_COUNT)
	fmt.Printf("Concurrency: %d\n", s.CONCURRENCY)
	fmt.Printf("Persistent Mode: %v\n", s.PERSISTENT)
	fmt.Printf("Total Time: %v\n", totalTime)
	fmt.Printf("Average Execution Time: %v\n", averageTime)
	fmt.Printf("Min Execution Time: %v\n", minTime)
	fmt.Printf("Max Execution Time: %v\n", maxTime)
	fmt.Printf("99th Percentile Execution Time: %v\n", p99Time)
}

func worker(key *agent.Key, taskCh <-chan struct{}, doneCh chan<- []time.Duration, persistent bool) {
	var executionTimes []time.Duration
	var err error
	var agentClient sshAgent
	for range taskCh {
		start := time.Now()
		if agentClient == nil {
			agentClient, err = newAgent()
			if err != nil {
				log.Fatal(err)
			}
		}
		data := []byte("Benchmark data")
		_, err := agentClient.Sign(key, data)
		if !persistent {
			agentClient.Close()
			agentClient = nil
		}
		if err != nil {
			log.Printf("Failed to sign data: %v", err)
			continue
		}
		duration := time.Since(start)
		executionTimes = append(executionTimes, duration)
		fmt.Print(".")
	}
	doneCh <- executionTimes
}
