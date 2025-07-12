package main

import (
	bitcask "bitcask-go"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
)

var db *bitcask.DB

func init() {
	// 初始化DB实例
	options := bitcask.DefaultOptions
	mkdirTemp, _ := os.MkdirTemp("", "bitcask-go-http")
	options.DirPath = mkdirTemp
	var err error
	db, err = bitcask.Open(options)
	if err != nil {
		panic(fmt.Sprintf("failed to open db: %v", err))
	}

}

func handlePut(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var kv map[string]string

	if err := json.NewDecoder(r.Body).Decode(&kv); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for key, value := range kv {
		if err := db.Put([]byte(key), []byte(value)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Printf("fail to put key: %s, value: %s, %v", key, value, err)
			return
		}
	}
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	key := r.URL.Query().Get("key")

	value, err := db.Get([]byte(key))
	if err != nil && !errors.Is(err, bitcask.ErrKeyNotFound) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("fail to get key: %s, %v", key, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"key":   key,
		"value": string(value),
	})
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	key := r.URL.Query().Get("key")
	err := db.Delete([]byte(key))
	if err != nil && !errors.Is(err, bitcask.ErrKeyIsEmpty) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("fail to delete key: %s, %v", key, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode("OK")
}

func handleListKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	keys := db.ListKeys()
	w.Header().Set("Content-Type", "application/json")
	var res []string
	for _, key := range keys {
		res = append(res, string(key))
	}
	_ = json.NewEncoder(w).Encode(res)
}

func handleStat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stat := db.Stat()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stat)
}

func main() {
	// 注册处理方法
	http.HandleFunc("/bitcask/put", handlePut)
	http.HandleFunc("/bitcask/get", handleGet)
	http.HandleFunc("/bitcask/delete", handleDelete)
	http.HandleFunc("/bitcask/list-keys", handleListKeys)
	http.HandleFunc("/bitcask/stat", handleStat)
	// 启动http服务
	_ = http.ListenAndServe("localhost:8080", nil)
}
