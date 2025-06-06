# ipfs_gin_example

## 拉依赖
```cmd
go get github.com/gin-gonic/gin
go get github.com/dgraph-io/badger/v4
go get github.com/multiformats/go-multihash # Optional, for real CID, but we'll use hex hash for simplicity first
go get github.com/mitchellh/go-homedir # For database path
```
## 启动

```
go编译运行
go build
go run main.go
```

```cmd
```

## 测试

1. 上传文件(test目录下), 数据存储在data下面

   ```cmd
   先注释掉mock.go mockMap中的hello字段，执行
   curl.exe -X PUT "http://localhost:8080/hello.com/home/hello.txt" --data-binary "@hello.txt"
   执行后解掉domain的hello字段的注释并修改cid为返回的新cid
   ```

   

2. 下载文件

   ```cmd
   curl.exe "http://localhost:8080/hello.com/home/hello.txt" --output download_hello.txt
   ```

   