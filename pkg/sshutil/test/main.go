package main

import (
	"encoding/pem"
	"log"
	"os"

	"golang.org/x/crypto/ssh"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	sshPrivateKeyPath := os.Args[1]
	keyData, err := os.ReadFile(sshPrivateKeyPath)
	if err != nil {
		log.Fatal(err)
	}
	signer, err := ssh.ParsePrivateKey([]byte(keyData))
	if err == nil {
		log.Fatal("no passphrase: ", string(signer.PublicKey().Type()))
	}
	log.Printf("ssh.ParsePrivateKey err:%s error type:%T", err, err)
	block, _ := pem.Decode(keyData)
	if block == nil {
		log.Fatal("ssh: no key found.")
	}
	log.Printf("block type:%s, Headers:%v", block.Type, block.Headers)

	passPhrase := os.Getenv("PAS")
	if passPhrase == "" {
		log.Fatal("Please configure `PASSPHRASE` environment variable for your passphrase")
	}

	signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(passPhrase))
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Parsed private key with passphrase!!")
	log.Println(string(signer.PublicKey().Type()))
}
