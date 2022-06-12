# GitHub's Markdown URL Validator

![](/img/logo.svg)

## Overview
`gmuv` stands for **G**itHub's **M**arkdown **U**RL **V**alidator. `gmuv` is an open-source CLI tool to check/validate broken/failure links in Markdown files(*.md) for public Github repositories under a certain account.

## Quick start

### Local installation

Go (requires ver. >= 1.18) and git installed on Linux OS:
```
go install github.com/groovy-sky/gmuv/v2@latest
```

### Using Docker image

```
docker run -it golang:latest
go install github.com/groovy-sky/gmuv/v2@latest
```

## Commands

To see available options run following command:
```
gmuv -h
```

To check and validate links under a specific account and write output to the console:
```
gmuv -u groovy-sky -o cli
```

## ToDo

* Learn how-to and write [tests](https://pkg.go.dev/testing)
* Publish a docker image to Github registry
* Improve code quality
* Learn how-to use [Codecov](https://app.codecov.io/gh/groovy-sky/gmuv)

~~* Choose which one CLI to use ([standard](https://pkg.go.dev/flag), [urfave/cli](https://github.com/urfave/cli), [spf13/cobra](https://github.com/spf13/cobra))~~

~~* Setup workflow for building packages for different platforms (386,amd64 etc.)~~


## License
This project is released under [the BSD-3-Clause license](https://github.com/groovy-sky/gmuc/blob/main/LICENSE).
