package main

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"time"
	"syscall"

	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"

	"github.com/senseyeio/service-wrapper/processor/vault-aws"
)

func main() {
	fmt.Printf("Starting wrapper")
	time.Sleep(15 * time.Second)
	httpc := &http.Client{}

	aws := vaultaws.New(httpc)

	awsConfig := aws.Config()
	envconfig.MustProcess("", awsConfig)
	time.Sleep(15 * time.Second)

	awsEnabled, err := aws.Init(awsConfig)

	if err != nil {
		fmt.Printf("init error: %s\n", errors.Wrap(err, "Failed to initialise Vault AWS Auth"))
	time.Sleep(60 * time.Second)
		panic("panic")
	}

	args := os.Args[1:]
	envv := os.Environ()

	if awsEnabled {
		var doErr error
		args, envv, doErr = aws.Apply(args, envv)
		if doErr != nil {
			fmt.Printf("apply error: %s\n", errors.Wrap(doErr, "Failed to create Vault Token"))
	time.Sleep(60 * time.Second)
			panic("panic")
		}
	}

	command := args[0]
	args[0] = path.Base(command)

	fmt.Printf("Running %s\n", args)
	time.Sleep(60 * time.Second)

	err = syscall.Exec(command, args, envv)
}
