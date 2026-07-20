package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/davidmz/go-pageant"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

var listFlag bool

func init() {
	flag.BoolVar(&listFlag, "L", false, "Lists fingerprints of all identities currently represented by the agent")
	flag.Parse()
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	ok := pageant.Available()
	if !ok {
		log.Fatal("pageant is not available")
	}

	p := pageant.New()

	if listFlag {
		keys, err := p.List()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("key list:")
		for i, k := range keys {
			fmt.Printf("%d: [%s]\n", i, k.String())
		}
		return
	}
	if len(flag.Args()) < 1 {
		log.Fatal("Enter the key file name in the option")
	}

	pemBytes, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	key, err := ssh.ParseRawPrivateKey(pemBytes)
	if err != nil {
		log.Fatal(err)
	}

	err = p.Add(agent.AddedKey{
		PrivateKey: key,
	})
	if err != nil {
		log.Fatal(err)
	}
}
