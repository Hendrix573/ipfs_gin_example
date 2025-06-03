# ipfs_gin_example

## 启动

```
go编译运行
go build
go run main.go
```

```cmd
```

## 测试

1. 上传文件(test下), 数据存储在data下面

   ```cmd
   Invoke-WebRequest -Uri http://localhost:8080/upload -Method POST -InFile hello.txt -ContentType "application/octet-stream"
   ```

   

2. 下载文件

   ```cmd
   Invoke-WebRequest -Uri http://localhost:8080/hello.com
   ```

   