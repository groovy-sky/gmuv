# GitHub's Markdown URL Validator

![](/img/logo.svg)

## Overview
`gmuv` stands for **G**itHub's **M**arkdown **U**RL **V**alidator. `gmuv` is an open-source CLI tool to check and report about broken/failure links in Markdown files(*.md) for public Github repositories under a certain account.

## Quick start

### Local run

Go (requires ver. >= 1.18) and git installed on Linux OS:
```
go install github.com/groovy-sky/gmuv/v2@latest
gmuv -u groovy-sky --run-only
```

### Docker

```
docker run -it golang:latest
go install github.com/groovy-sky/gmuv/v2@latest
gmuv -u groovy-sky --run-only
```

## Concepts


## Commands


## License
This project is released under [the BSD-3-Clause license](https://github.com/groovy-sky/gmuc/blob/main/LICENSE).
