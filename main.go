package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type Record struct {
	ID        int64  `json:"id"`
	Username  string `json:"name"`
	Score     int    `json:"id_card"`
	Telephone string `json:"phone"`
	Code1     string `json:"affiliation"`
	Code2     string `json:"additional_info"`
}

// Custom unmarshal method
func (r *Record) UnmarshalJSON(b []byte) error {
	var temp struct {
		ID        interface{} `json:"id"`
		Username  string      `json:"name"`
		Score     interface{} `json:"id_card"`
		Telephone string      `json:"phone"`
		Code1     interface{} `json:"affiliation"`
		Code2     interface{} `json:"additional_info"`
	}

	if err := json.Unmarshal(b, &temp); err != nil {
		return err
	}

	r.Username = temp.Username
	r.Telephone = temp.Telephone
	// Helper function to convert interface{} to int64
	toInt64 := func(value interface{}) (int64, error) {
		switch v := value.(type) {
		case string:
			return strconv.ParseInt(v, 10, 64)
		case float64:
			return int64(v), nil
		default:
			return 0, fmt.Errorf("invalid type")
		}
	}

	// Helper function to convert interface{} to int
	toInt := func(value interface{}) (int, error) {
		switch v := value.(type) {
		case string:
			out, err := strconv.ParseInt(v, 10, 32)
			return int(out), err
		case float64:
			return int(v), nil
		default:
			return 0, fmt.Errorf("invalid type")
		}
	}

	r.ID, _ = toInt64(temp.ID)
	r.Score, _ = toInt(temp.Score)
	r.Code1 = fmt.Sprintf("%v", temp.Code1)
	r.Code2 = fmt.Sprintf("%v", temp.Code2)

	return nil
}

func insertMockData() error {
	query := `INSERT INTO default.clickhouse_sec (id,name,id_card,phone,affiliation,additional_info) VALUES 
             (2,'hacker','3333','15534212521','15534212521','15534212521'),
             (1,'gacj','33333333','15534212521','1223321','hacker')`

	resp, err := http.Post("http://127.0.0.1:8123/", "text/plain", strings.NewReader(query))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("failed to insert mock data, status code: %d, body: %s", resp.StatusCode, body)
	}

	return nil
}

func createTableAndInsertData() error {
	if err := createTable(); err != nil {
		return err
	}

	if err := insertMockData(); err != nil {
		return err
	}

	return nil
}

func createTable() error {
	query := `CREATE TABLE default.clickhouse_sec (
               id Int64,
               name String,
               id_card String,
               phone String,
               affiliation String,
               additional_info String
             ) ENGINE = MergeTree() ORDER BY id`

	resp, err := http.Post("http://127.0.0.1:8123/", "text/plain", strings.NewReader(query))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("failed to create clickhouse_sec, status code: %d, body: %s", resp.StatusCode, body)
	}

	return nil
}

func checkTableExists(tableName string) (bool, error) {
	query := "desc default." + tableName
	resp, err := http.Post("http://127.0.0.1:8123/", "text/plain", strings.NewReader(query))
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	} else {
		return false, nil
	}
}

func main() {
	// 检查your_table2表是否存在
	tableExists, err := checkTableExists("clickhouse_sec")
	if err != nil {
		fmt.Printf("Failed to check table existence: %v\n", err)
		return
	}

	if !tableExists {
		if err := createTableAndInsertData(); err != nil {
			fmt.Printf("Failed to create table and insert data: %v\n", err)
			return
		}
	}

	// 第一个 Gin 引擎
	r := gin.Default()
	r.LoadHTMLGlob("templates/*")

	r.GET("/", func(c *gin.Context) {
		sortOrder := c.Query("sort")
		records, err := fetchRecords(sortOrder)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.HTML(http.StatusOK, "index.html", gin.H{"records": records})
	})

	// 使用 Goroutine 启动第一个服务器
	go func() {
		if err := r.Run(":8080"); err != nil {
			fmt.Printf("Failed to run server on port 8080: %v\n", err)
		}
	}()

	// 第二个 Gin 引擎
	r2 := gin.Default()
	r2.GET("/meta-data", func(c *gin.Context) {
		c.String(http.StatusOK, "flag{clickhouse}")
	})

	// 启动第二个服务器
	if err := r2.Run(":1111"); err != nil {
		fmt.Printf("Failed to run server on port 1111: %v\n", err)
	}
}

func fetchRecords(sortOrder string) ([]Record, error) {
	query := fmt.Sprintf("SELECT * FROM clickhouse_sec ORDER BY id %s FORMAT JSON", sortOrder)
	if matched, _ := regexp.MatchString(`(?i)select\s+\*+\s+from`, sortOrder); matched {
		return nil, fmt.Errorf("mamba out")
	}
	resp, err := http.Post("http://127.0.0.1:8123/", "text/plain", strings.NewReader(query))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch records, status code: %d, body: %s", resp.StatusCode, body)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 打印返回的数据来调试
	fmt.Println("Response Body:", string(body))

	// Process ClickHouse JSON response
	type ClickHouseResponse struct {
		Data []Record `json:"data"`
	}
	var chResp ClickHouseResponse
	if err := json.Unmarshal(body, &chResp); err != nil {
		return nil, err
	}

	return chResp.Data, nil
}
