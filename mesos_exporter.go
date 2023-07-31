package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"mesos/logutil"
	"mesos/metric"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "net/http/pprof"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

var (
	mu            sync.Mutex
	port          *int
	logpath       *string
	loglevel      *string
	interval      *int
	Limit         int64 = 200
	Weight        int64 = 1
	containsocket       = "GET /containers/json?all=true HTTP/1.0\r\n\r\n"
	statsSock           = "GET /containers/%s/stats?stream=false HTTP/1.0\r\n\r\n"
)

type DockerInfos struct {
	Id      string        `json:"id"`
	Names   []string      `json:"names"`
	Image   string        `json:"image"`
	Command string        `json:"command"`
	Created time.Duration `json:"created"`
	State   string        `json:"state"`
	Status  string        `json:"status"`
	Labels  struct {
		MESOS_TASK_ID string `json:"mesos_task_id"`
	}
	Stats DockerStats `json:"stats"`
}

type DockerStats struct {
	lg         *zap.Logger
	Read       string `json:"read"`
	Pids_stats struct {
		Current int `json:"current"`
	}
	BlkioStats map[string]interface{} `json:"blkio_stats"`
	Cpu_Stats  struct {
		Cpu_Usage struct {
			TotalUsage  int64   `json:"total_usage"`
			PercpuUsage []int64 `json:"percpu_usage"`
		}
		SystemCpuUsage int64 `json:"system_cpu_usage"`
	}
	Precpu_stats struct {
		Cpu_usage struct {
			TotalUsage  int64   `json:"total_usage"`
			PercpuUsage []int64 `json:"percpu_usage"`
		}
		SystemCpuUsage int64 `json:"system_cpu_usage"`
	}
	Memory_stats struct {
		Usage    int64 `json:"usage"`
		MaxUsage int64 `json:"max_usage"`
		Limit    int64 `json:"limit"`
	}
	Name     string  `json:"name"`
	Id       string  `json:"id"`
	CpuRate  float64 `json:"cpu_rate"`
	MemUse   int64   `json:"mem_use"`
	MemLimit int64   `json:"mem_limit"`
	MemRate  float64 `json:"mem_rate"`
	// stopped  chan struct{}
}

func getDockerClient() http.Client {
	transport := &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			// /var/run/docker.sock 为 docker 对外提供的 socket 路径
			return net.Dial("unix", "/var/run/docker.sock")
		},
	}

	// client := http.Client{
	// 	Transport: &http.Transport{
	// 		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
	// 			// /var/run/docker.sock 为 docker 对外提供的 socket 路径
	// 			return net.Dial("unix", "/var/run/docker.sock")
	// 		},
	// 	},
	// }
	client := http.Client{Transport: transport}
	defer client.CloseIdleConnections()
	defer transport.CloseIdleConnections()
	return client
}
func connDocker(lg *zap.Logger) (*net.UnixConn, error) {
	addr := net.UnixAddr{
		Name: "/var/run/docker.sock",
		Net:  "unix",
	}
	conn, err := net.DialUnix("unix", nil, &addr)
	if err != nil {
		lg.Error("conn docker socket error", zap.Error(err))
		return nil, err
	}
	err1 := conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err1 != nil {
		lg.Error("set conn write deadline error", zap.Error(err1))

	}
	// defer conn.Close()
	return conn, nil

}
func DockerRequest(reqeust string, lg *zap.Logger) ([]byte, error) {
	// mu.Lock()
	// defer mu.Unlock()
	conn, err := connDocker(lg)
	if err != nil {
		lg.Error("docker conn error", zap.Error(err))
		return nil, err
	}
	defer conn.Close()
	n, err := conn.Write([]byte(reqeust))
	if err != nil {
		lg.Error("conn write request error", zap.Error(err))
		return nil, err
	}
	lg.Debug("conn write", zap.Int("write length", n))

	res, err := ioutil.ReadAll(conn)
	if err != nil {
		lg.Error("read conn error", zap.Error(err))
		return nil, err
	}
	body := getBody(res[:])
	return body, nil

}

func getBody(result []byte) (body []byte) {
	for i := 0; i <= len(result)-4; i++ {
		if result[i] == 13 && result[i+1] == 10 && result[i+2] == 13 && result[i+3] == 10 {
			body = result[i+4:]
			break
		}
	}
	return
}

func ExecDockerClient(url string, lg *zap.Logger) ([]byte, error) {
	mu.Lock()
	defer mu.Unlock()
	client := getDockerClient()
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// body, err := ioutil.ReadAll(resp.Body)
	reader := bufio.NewReader(resp.Body)
	defer reader.Reset(nil)
	var data []byte
	buf := make([]byte, 512)
	for {
		b, err := reader.Read(buf)
		if err != nil && err != io.EOF {
			lg.Error("read byte resp body error", zap.Error(err))
			break
		}
		if b == 0 {
			break
		}
		data = append(data, buf[:b]...)
	}
	buf = nil

	return data, nil
}
func keep_decimal(num1 float64, num2 float64, lg *zap.Logger) float64 {
	res, err := strconv.ParseFloat(fmt.Sprintf("%.2f", num1/num2), 64)
	if err != nil {
		lg.Error("num1 decide num2 error", zap.Error(err))
	}
	return res
}

func DataVisual(dis float64, lg *zap.Logger) string {
	n := 0
	for dis >= 1000 {
		dis = keep_decimal(dis, 1024, lg)
		n++
	}
	switch n {
	case 1:
		return strconv.FormatFloat(dis, 'f', 2, 64) + "KiB"
	case 2:
		return strconv.FormatFloat(dis, 'f', 2, 64) + "MiB"
	case 3:

		return strconv.FormatFloat(dis, 'f', 2, 64) + "GiB"
	case 4:
		return strconv.FormatFloat(dis, 'f', 2, 64) + "TiB"
	default:
		return strconv.FormatFloat(dis, 'f', 2, 64) + "byte"

	}

}

func (ds *DockerStats) GetDockerStats() {
	cpudelta := ds.Cpu_Stats.Cpu_Usage.TotalUsage - ds.Precpu_stats.Cpu_usage.TotalUsage
	systemdelta := ds.Cpu_Stats.SystemCpuUsage - ds.Precpu_stats.SystemCpuUsage
	num_per := len(ds.Cpu_Stats.Cpu_Usage.PercpuUsage)
	ds.CpuRate = keep_decimal(float64(cpudelta)*float64(num_per)*100, float64(systemdelta), ds.lg)
	ds.MemUse = ds.Memory_stats.Usage
	ds.MemLimit = ds.Memory_stats.Limit
	ds.MemRate = keep_decimal(float64(ds.Memory_stats.Usage)*100, float64(ds.Memory_stats.Limit), ds.lg)

}

//	func (ds *DockerStats) onstop() {
//		ds.stopped <- struct{}{}
//	}
func regxMatchIntToTime(str string, lg *zap.Logger) int64 {
	reg := regexp.MustCompile(`[0-9]+`)
	h, err := strconv.ParseInt(reg.FindString(str), 10, 64)
	if err != nil {
		lg.Error("string convert to int64 failed", zap.Error(err))
	}
	return h
}
func formatStrtoInt64(str string, lg *zap.Logger) float64 {
	var res int64
	if strings.Contains(str, "hours") {
		r := regxMatchIntToTime(str, lg)
		res = r * 3600
	} else if strings.Contains(str, "days") {
		r := regxMatchIntToTime(str, lg)
		res = r * 3600 * 24
	} else if strings.Contains(str, "minutes") {
		r := regxMatchIntToTime(str, lg)
		res = r * 60
	} else {
		r := regxMatchIntToTime(str, lg)
		res = r
	}
	return float64(res)
}

func worker(ctx context.Context, resquest string, dk DockerInfos, stopch chan bool, interval int, lg *zap.Logger) {
	defer close(stopch)
	for {
		select {
		case <-ctx.Done():
			return
		case <-stopch:
			return
		default:
			res_stats, err := DockerRequest(resquest, lg)
			if err != nil {
				lg.Error("docker request error", zap.String("request", resquest), zap.Error(err))

			}
			stats := DockerStats{}
			if err := json.Unmarshal(res_stats, &stats); err != nil {
				lg.Error("res_stats unmarshl to stats error", zap.Error(err))
			}
			stats.GetDockerStats()
			dk.Stats = stats
			metric.CpuRate.WithLabelValues(dk.Id, dk.Names[0], dk.Image).Set(dk.Stats.CpuRate)
			metric.MemUse.WithLabelValues(dk.Id, dk.Names[0], dk.Image).Set(float64(dk.Stats.MemUse))
			metric.MemLimit.WithLabelValues(dk.Id, dk.Names[0], dk.Image).Set(float64(dk.Stats.MemLimit))
			metric.MemRate.WithLabelValues(dk.Id, dk.Names[0], dk.Image).Set(dk.Stats.MemRate)
			// if dk.State == "running" {
			// 	metric.State.WithLabelValues(dk.Id, dk.Names[0], dk.Image).Set(1)
			// } else {
			// 	metric.State.WithLabelValues(dk.Id, dk.Names[0], dk.Image).Set(0)
			// }
			// metric.Status.WithLabelValues(dk.Id, dk.Names[0], dk.Image).Set(formatStrtoInt64(dk.Status, lg))
			// metric.Status.WithLabelValues(dk.Id, dk.Names[0], dk.Image).Set(formatStrtoInt64(dk.Status, lg))
			lg.Debug("更新docker指标信息", zap.String("docker id", dk.Id), zap.String("docker stats id", dk.Stats.Id))
			dk.Stats = DockerStats{}
			if interval > 0 {
				dur := time.Duration(interval)
				time.Sleep(dur * time.Second)
			}

		}

	}

}

func watchContainers(ctx context.Context, flag chan bool, dockerMap map[string]DockerInfos, containerMap map[string]chan bool, lg *zap.Logger) {
	// Docker API 提供了一个 /events API，可以订阅 Docker 的事件
	// resp, err := DockerRequest("GET /events HTTP/1.0\r\n\r\n", lg)
	// fmt.Println("+++++")
	// if err != nil {
	// 	lg.Error("docker request events error", zap.Error(err))
	// 	return
	// }

	// eventCh := make(chan events.Message)
	// errCh := make(chan error)
	// defer close(eventCh)
	// defer close(errCh)
	// ctx1, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// defer cancel()

	// 将 Docker 事件流解码为 events.Message 对象
	// dec := json.NewDecoder(bytes.NewReader(resp))
	// go func() {
	// 	for {
	// 		select {
	// 		case <-ctx1.Done():
	// 			fmt.Println(2222)
	// 			return
	// 		default:
	// 			resp, err := DockerRequest("GET /events HTTP/1.0\r\n\r\n", lg)
	// 			fmt.Print(111111111111111)
	// 			if err != nil {
	// 				lg.Error("docker request events error", zap.Error(err))
	// 				return
	// 			}
	// 			dec := json.NewDecoder(bytes.NewReader(resp))
	// 			var msg events.Message
	// 			if err := dec.Decode(&msg); err != nil {
	// 				lg.Error("decode events messages error", zap.Error(err))
	// 				errCh <- err
	// 				return
	// 			}
	// 			eventCh <- msg
	// 			time.Sleep(30 * time.Second)
	// 		}
	// 	}

	// }()

	// dockerMap := make(map[string]*DockerInfos)
	ticker := time.NewTicker(30 * time.Second)
	// ticker2 := time.NewTicker(10 * time.Minute)
	// resc,err := DockerRequest(containsocket,lg)
	// if err != nil {
	// 	lg.Error("docker request error", zap.Error(err))
	// }
	// dock
	for {
		select {
		case <-flag:
			for id, info := range dockerMap {
				stopCh := make(chan bool)
				// defer close(stopCh)
				containerMap[id] = stopCh
				lg.Info("init starting docker stats goroutine", zap.String("dockerID", id), zap.String("goroutineID", id))
				go worker(ctx, fmt.Sprintf(statsSock, id), info, containerMap[id], *interval, lg)
			}
		case <-ticker.C:
			// 定时获取容器列表，更新本地容器信息
			resp, err := DockerRequest(containsocket, lg)
			if err != nil {
				lg.Error("docker request error", zap.Error(err))
				continue
			}

			var dockerinfo []DockerInfos
			if err := json.Unmarshal(resp, &dockerinfo); err != nil {
				lg.Error("json unmarshal failed", zap.Error(err))
				continue
			}
			for _, info := range dockerinfo {
				id := info.Id
				//更新容器基础信息
				if info.State == "running" {
					metric.State.WithLabelValues(info.Id, info.Names[0], info.Image).Set(1)
					metric.Status.WithLabelValues(info.Id, info.Names[0], info.Image).Set(formatStrtoInt64(info.Status, lg))
				} else {
					metric.State.WithLabelValues(info.Id, info.Names[0], info.Image).Set(0)
					metric.Status.WithLabelValues(info.Id, info.Names[0], info.Image).Set(0)
				}
				// 如果容器不存在，则添加到本地容器列表中
				if _, ok := dockerMap[id]; !ok {
					dockerMap[id] = info
					stopCh := make(chan bool)
					containerMap[id] = stopCh
					// defer close(stopCh)
					lg.Info("check new docker started,new docker container start", zap.String("dockerID", id))
					go worker(ctx, fmt.Sprintf(statsSock, id), info, containerMap[id], *interval, lg)
				}
			}

			// 如果容器在本地容器列表中但已被删除，则将其从本地容器列表中删除
			for id := range dockerMap {
				found := false
				for _, info := range dockerinfo {
					if id == info.Id {
						found = true
						break
					}
				}
				if !found {
					lg.Info("check found unused docker container deleting", zap.String("dockerID", id), zap.String("goroutineID", id))
					close(containerMap[id])
					delete(dockerMap, id)
				}
			}

			// case msg := <-eventCh:
			// 	switch msg.Type {
			// 	case events.ContainerEventType:
			// 		id := msg.Actor.ID
			// 		switch msg.Action {
			// 		case "create":
			// 			// 容器创建事件，根据 ID 添加到本地容器列表中
			// 			resp, err := DockerRequest(fmt.Sprintf("GET /containers/%s/json HTTP/1.0\r\n\r\n", id), lg)
			// 			if err != nil {
			// 				lg.Error("docker request error", zap.String("request", fmt.Sprintf("GET /containers/%s/json HTTP/1.0\r\n\r\n", id)), zap.Error(err))
			// 				continue
			// 			}

			// 			var info DockerInfos
			// 			if err := json.Unmarshal(resp, &info); err != nil {
			// 				lg.Error("json unmarshal failed", zap.Error(err))
			// 				continue
			// 			}
			// 			dockerMap[id] = info
			// 			stopCh := make(chan bool)
			// 			containerMap[id] = stopCh
			// 			// defer close(stopCh)
			// 			lg.Info("receive event notications,new docker container start", zap.String("dockerID", id))
			// 			go worker(ctx, fmt.Sprintf(statsSock, id), info, containerMap[id], *interval, lg)

			// 		case "destroy":
			// 			// 容器销毁事件，从本地容器列表中删除相应的容器
			// 			lg.Info("receive event notications,unused docker container destorying", zap.String("dockerID", id), zap.String("goroutineID", id))
			// 			close(containerMap[id])
			// 			delete(dockerMap, id)
			// 		}
			// 	}

			// case err := <-errCh:
			// 	lg.Error("docker event stream error", zap.Error(err))
			// 	return

			// case <-ctx.Done():
			// 	return
		}
	}
}

func init() {
	port = flag.Int("port", 9029, "metrics run port")
	logpath = flag.String("logpath", "logs/metric.log", "logpath default in current directroy logs")
	loglevel = flag.String("level", "info", "choose one of debug info error")
	interval = flag.Int("interval", 2, "update docker stats metrics interval")
}

func main() {
	// var wg sync.WaitGroup
	flag.Parse()
	// if *showVersion {
	// 	fmt.Println(version)
	// 	os.Exit(0)

	// }
	// var imagesSock = "GET /images/json HTTP/1.0\r\n\r\n"
	// var containsocket = "GET /containers/json?all=true HTTP/1.0\r\n\r\n"
	// var statsSock = "GET /containers/%s/stats?stream=false HTTP/1.0\r\n\r\n"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	lg := logutil.Logger(*logpath, *loglevel)
	resp, err := DockerRequest(containsocket, lg)
	if err != nil {
		lg.Error("docker request containsocket error", zap.Error(err))
	}
	var dockerinfo []DockerInfos
	if err := json.Unmarshal(resp, &dockerinfo); err != nil {
		lg.Error("json unmarshal failed", zap.Error(err))
	}

	// fmt.Println(dockerinfo)
	// ch := make(chan DockerInfos)
	// // sem := semaphore.NewWeighted(Limit)
	// // throttle := make(chan struct{}, 200)
	// ticker := time.NewTicker(5 * time.Second)
	// defer ticker.Stop()
	// stopCh := make(chan bool)
	containerMap := make(map[string]chan bool)
	// defer close(stopCh)
	// for _, i := range dockerinfo {
	// 	lg.Info("docker info长度", zap.Int("docker_info_length", len(dockerinfo)))
	// 	stats_url := fmt.Sprintf(statsSock, i.Id)
	// 	// 	ch <- i
	// 	lg.Info("goroutine starting", zap.String("dockerID", i.Id))
	// 	stopChan := make(chan bool)
	// 	containerMap[i.Id] = stopChan
	// 	go worker(ctx, stats_url, i, stopChan, *interval, lg)
	// }
	flag := make(chan bool, 1)
	flag <- true
	dockerMap := make(map[string]DockerInfos)
	for _, info := range dockerinfo {
		dockerMap[info.Id] = info
	}
	lg.Info("init get docker containers list", zap.Int("dockerNums", len(dockerMap)))
	go watchContainers(ctx, flag, dockerMap, containerMap, lg)

	// // wg.Wait()
	server_port := fmt.Sprintf(":%d", *port)
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(server_port, nil)

}
