package cmd

import (
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/xzb/pkg/db"
)

//go:embed index.html
var indexHTML []byte

// getDatabasePath returns the database path
func getDatabasePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// 如果无法获取用户主目录，则使用当前目录
		return "xzb.db"
	}
	// 默认数据库路径为 $HOME/.cache/xzb/xzb.db
	return filepath.Join(homeDir, ".cache", "xzb", "xzb.db")
}

// createWebCmd creates the web command
func createWebCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "web",
		Short: "启动Web服务查询账单数据",
		Long:  `启动一个Web服务，可以通过API查询账单数据或在浏览器中查看账单`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// 检查数据库文件是否存在
			dbPath := getDatabasePath()
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				return fmt.Errorf("数据库文件不存在，请先运行 merge 命令导入数据")
			}

			// 获取端口参数
			port, _ := cmd.Flags().GetString("port")

			// 注册路由
			http.HandleFunc("/api/bills", billsHandler)
			http.HandleFunc("/api/summary", summaryHandler)
			http.HandleFunc("/", indexHandler)

			log.Printf("Web界面: http://localhost:%s", port)

			server := &http.Server{
				Addr:         ":" + port,
				ReadTimeout:  10 * time.Second,
				WriteTimeout: 10 * time.Second,
				IdleTimeout:  30 * time.Second,
			}
			return server.ListenAndServe()
		},
	}
}

// billsHandler 处理账单查询请求
func billsHandler(w http.ResponseWriter, r *http.Request) {
	// 设置CORS头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	// 检查数据库文件
	dbPath := getDatabasePath()
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		http.Error(w, "数据库文件不存在", http.StatusNotFound)
		return
	}

	// 获取查询参数
	conditions := make(map[string]interface{})

	startDate := r.URL.Query().Get("start_date")
	if startDate != "" {
		conditions["start_date"] = startDate
	}

	endDate := r.URL.Query().Get("end_date")
	if endDate != "" {
		conditions["end_date"] = endDate
	}

	category := r.URL.Query().Get("category")
	if category != "" {
		conditions["category"] = category
	}

	inOut := r.URL.Query().Get("in_out")
	if inOut != "" {
		conditions["in_out"] = inOut
	}

	accountType := r.URL.Query().Get("account_type")
	if accountType != "" {
		conditions["account_type"] = accountType
	}

	counterparty := r.URL.Query().Get("counterparty")
	if counterparty != "" {
		conditions["counterparty"] = counterparty
	}

	// 查询数据
	records, err := db.QueryRecords(dbPath, conditions)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 返回JSON数据
	if err := db.EncodeJSON(w, records); err != nil {
		http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
	}
}

// summaryHandler 处理汇总信息请求
func summaryHandler(w http.ResponseWriter, _ *http.Request) {
	// 设置CORS头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	// 检查数据库文件
	dbPath := getDatabasePath()
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		http.Error(w, "数据库文件不存在", http.StatusNotFound)
		return
	}

	// 获取所有记录
	records, err := db.GetAllRecords(dbPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 计算汇总信息
	summary := db.CalculateSummary(records)

	// 返回JSON数据
	if err := db.EncodeJSON(w, summary); err != nil {
		http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
	}
}

// indexHandler 处理主页请求
func indexHandler(w http.ResponseWriter, r *http.Request) {
	// 如果不是根路径，返回404
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// 检查数据库文件
	dbPath := getDatabasePath()
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		http.Error(w, "数据库文件不存在，请先运行 merge 命令导入数据", http.StatusNotFound)
		return
	}

	// 设置内容类型
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// 直接写入嵌入的HTML内容
	if _, err := w.Write(indexHTML); err != nil {
		http.Error(w, "Failed to write response", http.StatusInternalServerError)
	}
}

// getWebCmd returns the web command for external registration
func getWebCmd() *cobra.Command {
	webCmd := createWebCmd()
	webCmd.Flags().StringP("port", "p", "8080", "Web服务端口")
	return webCmd
}
