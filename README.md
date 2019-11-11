# service-wrapper
A wrapper to modify the launch conditions of a dockerized processor. The
primary motivations for this are:

* small footprint for lightweight containers (eg based on scratch)
* ability to deploy a single container in different environments

## Vault AWS

Triggered by the presence of the environmental variable `VAULT_ADDR` and
`AWS_CONTAINER_CREDENTIALS_RELATIVE_URI`. The former is a config parameter
and the latter is provided by AWS ECS when a task starts and the service
in question has a IAM task role attached. The credentials provided by
the `AWS_CONTAINER_CREDENTIALS_RELATIVE_URI` endpoint will be used to
authenticate with Vault using Vault's AWS Auth method (sepecifically
the IAM flavour). Details about the AWS endpoint can be found in the AWS
ECS IAM Task role documentation. If Vault authenticates the credentials,
then the resulting token is passed to envconsul.  It is preferrable for
this token to be a cubbyhole wrapped response token as it will be passed
on the commandline to the envconsul.

If `VAULT_SERVICE_FILE` is defined then envconsul will be launched
with this file as its config. If `VAULT_SERVICE_CONFIG` is defined then
this processor will use the contents of the variable as a standard Go
template and write the contents to the `VAULT_SERVICE_FILE` file. The
template is provided the process's environment variables as a map. The
file would ideally exist on a tmpfs mount. If no `VAULT_SERVICE_CONFIG`
is defined it is expected that `VAULT_SERVICE_FILE` points to a file
that is already present on the container.

Note that the address of the vault server and credentials are passed in
via the command line and do not need to be specified in the config file.

An example `VAULT_SERVICE_CONFIG`:
```
exec {
  env {
    blacklist = ["VAULT_*"]
  }
  kill_timeout = "20s"
}

secret {
  path = "kv/common"
  no_prefix = true
}

secret {
  path = "kv/{{.SERVICE_NAME}}"
  no_prefix = true
}
```

Container requirements:
* `envconsul` to be installed in `/bin/consul`
