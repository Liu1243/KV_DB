# KV DB
基于bitcask实现的持久性KV存储

## benchmark
`go test -bench=. -benchtime=5s`
![img_2.png](img_2.png)
`go test -bench=. -benchtime=1000000x`
![img_1.png](img_1.png)