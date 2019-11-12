package main

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"syscall"

	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"

	"github.com/senseyeio/service-wrapper/processor/vault-aws"
)

func main() {
	fmt.Printf("Starting service-wrapper")
	httpc := &http.Client{}

	aws := vaultaws.New(httpc)

	awsConfig := aws.Config()
	envconfig.MustProcess("", awsConfig)

	awsEnabled, err := aws.Init(awsConfig)

	if err != nil {
		panic(errors.Wrap(err, "Failed to initialise Vault AWS Auth"))
	}

	args := os.Args[1:]
	envv := os.Environ()

	if awsEnabled {
		var doErr error
		args, envv, doErr = aws.Apply(args, envv)
		if doErr != nil {
			panic(errors.Wrap(doErr, "Failed to create Vault Token"))
		}
	}

	command := args[0]
	args[0] = path.Base(command)

	err = syscall.Exec(command, args, envv)
}
