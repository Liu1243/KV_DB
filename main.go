package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	tempDir := os.TempDir()

	// 只读文件
	readOnlyFile, err := os.OpenFile(filepath.Join(tempDir, "readonly.txt"), os.O_CREATE|os.O_RDONLY, 0400)
	if err != nil {
		panic(err)
	}
	defer readOnlyFile.Close()

	n, err := readOnlyFile.Write([]byte("test"))
	fmt.Println(n, err) // 在 Linux 下通常会输出：0 和 "bad file descriptor"
}
