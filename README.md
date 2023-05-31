# Terraform Provider F5OS

* F5OS Terraform provider for F5 VELOS and F5 rSeries helps you automate configurations and interactions with various services provided by F5 VELOS platform and F5 rSeries appliances

* F5 VELOS platforms are next generation industry-leading chassis-based systems, designed to meet the needs of large enterprise networking environments that require the ability to scale and process a large volume of increasing application workloads.

* F5 rSeries platforms are powerful systems that are designed specifically for application delivery performance and scalability

  For more information:

  [F5 VELOS hardware](https://www.f5.com/products/big-ip-services/velos-hardware-chassis-and-blades) and [system overview](https://techdocs.f5.com/en-us/velos-1-5-0/velos-systems-administration-configuration/title-velos-system-overview.html#intro-velos-systems)

  [F5 rSeries hardware](https://www.f5.com/products/big-ip-services/rseries-adc-hardware-appliance) and [system overview](https://techdocs.f5.com/en-us/hardware/f5-rseries-systems-getting-started.html)


## Requirements

* [Terraform](https://www.terraform.io/downloads) > 1.x
* [Go](https://go.dev/doc/install) >= 1.19
* [GNU Make](https://www.gnu.org/software/make/)
* [golangci-lint](https://golangci-lint.run/usage/install/#local-installation) (optional)

## Using the Provider

This Terraform Provider is available to install automatically via `terraform init`. It is recommended to setup the following Terraform configuration to pin the major version:

```hcl
# Terraform 1.2.x and later
terraform {
  required_providers {
    f5os = {
      source  = "f5networks/f5os"
      version = "~> X.Y" # where X.Y is the current major version and minor version
    }
  }
}
```

## Documentation, questions and discussions
Official documentation on how to use this provider can be found on the
[Terraform Registry](https://registry.terraform.io/providers/F5Networks/f5os/latest/docs).
In case of specific questions or discussions, please use the
HashiCorp [Terraform Providers Discuss forums](https://discuss.hashicorp.com/c/terraform-providers/31),
in accordance with HashiCorp [Community Guidelines](https://www.hashicorp.com/community-guidelines).

We also provide:

* [Support](.github/SUPPORT.md) page for help when using the provider
* [Contributing](.github/CONTRIBUTING.md) guidelines in case you want to help this project

## Compatibility

Compatibility table between this provider, the [Terraform Plugin Protocol](https://www.terraform.io/plugin/how-terraform-works#terraform-plugin-protocol)
version it implements, and Terraform:

| F5OS Provider |     Terraform Plugin Protocol      | Terraform | F5OS Velos/rSeries Version |
|:-------------:|:----------------------------------:|:---------:|:--------------------------:|
|  `>= 1.0.0`   |                `6`                 | `>= 1.x`  |      `>= 1.5.1/1.4.0`      |

Details can be found querying the [Registry API](https://www.terraform.io/internals/provider-registry-protocol#list-available-versions)
that return all the details about which version are currently available for a particular provider.

## Development

### Building

1. `git clone` this repository and `cd` into its directory
2. `go build` will trigger the Golang build

The provided `GNUmakefile` defines additional commands generally useful during development,
like for running tests, generating documentation, code formatting and linting.
Taking a look at it's content is recommended.

### Testing

In order to test the provider, you can run

* `make test` to run provider unit tests
* `make testacc` to run provider acceptance tests

It's important to note that acceptance tests (`testacc`) will actually spawn real resources, and often cost money to run. Read more about they work on the
[official page](https://www.terraform.io/plugin/sdkv2/testing/acceptance-tests).

### Generating documentation

This provider uses [terraform-plugin-docs](https://github.com/hashicorp/terraform-plugin-docs/)
to generate documentation and store it in the `docs/` directory.
Once a release is cut, the Terraform Registry will download the documentation from `docs/`
and associate it with the release version. Read more about how this works on the
[official page](https://www.terraform.io/registry/providers/docs).

Use `make generate` to ensure the documentation is regenerated with any changes.

### Using a development build

If [running tests and acceptance tests](#testing) isn't enough, it's possible to set up a local terraform configuration
to use a development builds of the provider. This can be achieved by leveraging the Terraform CLI
[configuration file development overrides](https://www.terraform.io/cli/config/config-file#development-overrides-for-provider-developers).

First, use `make install` to place a fresh development build of the provider in your
[`${GOBIN}`](https://pkg.go.dev/cmd/go#hdr-Compile_and_install_packages_and_dependencies)
(defaults to `${GOPATH}/bin` or `${HOME}/go/bin` if `${GOPATH}` is not set). Repeat
this every time you make changes to the provider locally.

Then, setup your environment following [these instructions](https://www.terraform.io/plugin/debugging#terraform-cli-development-overrides)
to make your local terraform use your local build.

*Note:* Acceptance tests create real resources, and often cost money to run.

```sh
$ make testacc
```