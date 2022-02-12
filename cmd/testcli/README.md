
## build

```bash
# install 'syso' 
go get -u github.com/hallazzang/syso/...

# build out.syso
syso

# build
GOOS=windows go build -ldflags="-H windowsgui"
```
