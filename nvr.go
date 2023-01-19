// Copyright 2020-2022 The OS-NVR Authors.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package nvr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"net"
	"net/http"
	"nvr/pkg/group"
	"nvr/pkg/log"
	"nvr/pkg/monitor"
	"nvr/pkg/storage"
	"nvr/pkg/system"
	"nvr/pkg/video"
	"nvr/pkg/web"
	"nvr/pkg/web/auth"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/avast/retry-go"
)

var RegisterUrl string
var UnRegisterUrl string
var SyncUrl string
var OsnvrId string

type Osnvr struct {
	Id         string `json:"id" validate:"required"`
	GroupId    string `json:"groupid,omitempty"`
	ServerPort string `json:"serverport,omitempty" validate:"required"`
	RtspPort   string `json:"rtspport,omitempty" validate:"required"`
	HlsPort    string `json:"hlsport,omitempty" validate:"required"`
	Desc       string `json:"desc,omitempty"`
}

// Run .
func Run() error {
	envFlag := flag.String("env", "", "path to env.yaml")
	flag.Parse()

	if *envFlag == "" {
		flag.Usage()
		return nil
	}

	envPath, err := filepath.Abs(*envFlag)
	if err != nil {
		return fmt.Errorf("could not get absolute path of env.yaml: %w", err)
	}

	wg := &sync.WaitGroup{}
	app, err := newApp(envPath, wg, hooks)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fatal := make(chan error, 1)
	go func() { fatal <- app.run(ctx) }()

	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be catch, so don't need add it
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	if os.Getenv("POD_NAME") == "" {
		OsnvrId, err = ExternalIP()
	} else {
		OsnvrId = os.Getenv("POD_NAME") + ".osnvr"
	}

	if os.Getenv("OSNVRAPIURL") == "" {
		RegisterUrl = "http://localhost:6060/api/v1/osnvr"
		UnRegisterUrl = "http://localhost:6060/api/v1/osnvr" + "/" + OsnvrId
		SyncUrl = "http://localhost:6060/api/v1/osnvr" + "/sync/" + OsnvrId
	} else {
		RegisterUrl = os.Getenv("OSNVRAPIURL")
		UnRegisterUrl = os.Getenv("OSNVRAPIURL") + "/" + OsnvrId
		SyncUrl = os.Getenv("OSNVRAPIURL") + "/sync/" + OsnvrId
	}

	/*************** Register OS-NVR ***************/
	client := &http.Client{}
	osnvr := Osnvr{
		GroupId:    "1",
		Id:         OsnvrId,
		ServerPort: fmt.Sprintf("%d", app.Env.Port),
		RtspPort:   fmt.Sprintf("%d", app.Env.RTSPPort),
		HlsPort:    fmt.Sprintf("%d", app.Env.HLSPort),
		Desc:       "automatically registered",
	}

	data, _ := json.Marshal(osnvr)
	req, err := http.NewRequest(http.MethodPost, RegisterUrl, bytes.NewBuffer(data))
	res, err := client.Do(req)
	if err != nil {
		app.logf(log.LevelError, "register osnvr error: %v. It cannot do register with VMS API.", err)
	} else {
		fmt.Println("") // New line.
		app.logf(log.LevelInfo, "register osnvr: %s succesfully.", OsnvrId)
		res.Body.Close()
	}
	/*********************************************/

	/********** Sync Nvrconfigs back to OSNVR Instance **********/
	// By default to retry for 10 times by using retry library
	req, err = http.NewRequest(http.MethodGet, SyncUrl, nil)
	var body []byte
	var msg map[string]interface{}
	retry.Do(
		func() error {
			res, err = client.Do(req)
			if err != nil {
				app.logf(log.LevelError, "sync osnvr error: %v. It cannot sync with VMS API.", err)
				return err
			}
			body, err = ioutil.ReadAll(res.Body)
			if err != nil {
				return err
			}
			err = json.Unmarshal(body, &msg)
			if msg["message"] == "error" {
				app.logf(log.LevelError, "message:%s", msg["message"])
				return err
			}
			fmt.Println("") // New line.
			app.logf(log.LevelInfo, "sync osnvr: %s succesfully.", OsnvrId)
			defer res.Body.Close()
			return nil
		},
	)
	/*********************************************/

	select {
	case err = <-fatal:
		app.logf(log.LevelError, "fatal error: %v", err)
	case signal := <-stop:
		fmt.Println("") // New line.
		app.logf(log.LevelInfo, "received %v, stopping", signal)
	}

	/*** Un-register OS-NVR ***/
	fmt.Println("UnRegisterUrl:", UnRegisterUrl)
	req, _ = http.NewRequest(http.MethodDelete, UnRegisterUrl, nil)
	res, err = client.Do(req)
	if err != nil {
		app.logf(log.LevelError, "un-register osnvr: %s error: %v", OsnvrId, err)
	}
	/*** The end of Un-register OS-NVR ***/

	app.monitorManager.StopMonitors()
	app.logf(log.LevelInfo, "Monitors stopped.")

	cancel()
	wg.Wait()

	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()

	if err != nil {
		return err
	}
	return app.server.Shutdown(ctx2)
}

// App is the main application.
type App struct {
	WG             *sync.WaitGroup
	Logger         *log.Logger
	logStore       *log.Store
	Env            storage.ConfigEnv
	monitorManager *monitor.Manager
	Auth           auth.Authenticator
	Storage        *storage.Manager
	videoServer    *video.Server
	Templater      *web.Templater
	Router         *http.ServeMux
	server         *http.Server
}

func newApp(envPath string, wg *sync.WaitGroup, hooks *hookList) (*App, error) { //nolint:funlen
	// Environment config.
	envYAML, err := os.ReadFile(envPath)
	if err != nil {
		return nil, fmt.Errorf("could not read env.yaml: %w", err)
	}

	env, err := storage.NewConfigEnv(envPath, envYAML)
	if err != nil {
		return nil, fmt.Errorf("could not get environment config: %w", err)
	}

	/*************** Clean up nvr monitor configs *****************/
	cPath := filepath.Join(env.HomeDir, "configs/monitors/")
	fmt.Printf("nvr monitor config:%s\n", cPath)
	os.RemoveAll(cPath)
	/**************************************************************/

	general, err := storage.NewConfigGeneral(env.ConfigDir)
	if err != nil {
		return nil, fmt.Errorf("could not get general config: %w", err)
	}

	// Logs.
	logDir := filepath.Join(env.StorageDir, "logs")
	logger := log.NewLogger(wg, hooks.logSource)
	logStore, err := log.NewStore(logDir, wg, general.DiskSpace)
	if err != nil {
		return nil, fmt.Errorf("could not create log store: %w", err)
	}

	// Video server.
	videoServer := video.NewServer(logger, wg, *env)

	// Monitors.
	monitorConfigDir := filepath.Join(env.ConfigDir, "monitors")
	monitorManager, err := monitor.NewManager(
		monitorConfigDir,
		*env,
		logger,
		videoServer,
		hooks.monitor(),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create monitor manager: %w", err)
	}

	// Monitor groups.
	groupConfigDir := filepath.Join(env.ConfigDir, "groups")
	groupManager, err := group.NewManager(groupConfigDir)
	if err != nil {
		return nil, fmt.Errorf("could not create monitor manager: %w", err)
	}

	// Authentication.
	if hooks.newAuthenticator == nil {
		return nil, fmt.Errorf( //nolint:goerr113
			"no authentication addon enabled, please enable one in '%v'", envPath)
	}

	a, err := hooks.newAuthenticator(*env, logger)
	if err != nil {
		return nil, fmt.Errorf("could not create authenticator: %w", err)
	}

	// Storage.
	storageManager := storage.NewManager(env.StorageDir, general, logger)
	crawler := storage.NewCrawler(os.DirFS(storageManager.RecordingsDir()))

	// Time zone.
	timeZone, err := system.TimeZone()
	if err != nil {
		return nil, err
	}

	// Templates.
	t, err := web.NewTemplater(a, hooks.tplHooks())
	if err != nil {
		return nil, err
	}

	t.RegisterTemplateDataFuncs(
		func(data template.FuncMap, _ string) {
			data["theme"] = general.Get()["theme"]
		},
		func(data template.FuncMap, _ string) {
			data["tz"] = timeZone
		},
		func(data template.FuncMap, page string) {
			groups, _ := json.Marshal(groupManager.Configs())
			data["groups"] = string(groups)
		},
		func(data template.FuncMap, page string) {
			monitors, _ := json.Marshal(monitorManager.MonitorsInfo())
			data["monitors"] = string(monitors)
		},
		func(data template.FuncMap, page string) {
			data["logSources"] = logger.Sources()
		},
	)
	t.RegisterTemplateDataFuncs(hooks.templateData...)

	// Routes.
	router := http.NewServeMux()

	router.Handle("/live", a.User(t.Render("live.tpl")))
	router.Handle("/recordings", a.User(t.Render("recordings.tpl")))
	router.Handle("/settings", a.User(t.Render("settings.tpl")))
	router.Handle("/settings.js", a.User(t.Render("settings.js")))
	router.Handle("/logs", a.Admin(t.Render("logs.tpl")))
	router.Handle("/debug", a.Admin(t.Render("debug.tpl")))

	router.Handle("/static/", a.User(web.Static()))
	router.Handle("/hls/", a.User(videoServer.HandleHLS()))

	router.Handle("/api/system/time-zone", a.User(web.TimeZone(timeZone)))

	router.Handle("/api/general", a.Admin(web.General(general)))
	router.Handle("/api/general/set", a.Admin(a.CSRF(web.GeneralSet(general))))

	router.Handle("/api/users", a.Admin(web.Users(a)))
	router.Handle("/api/user/set", a.Admin(a.CSRF(web.UserSet(a))))
	router.Handle("/api/user/delete", a.Admin(a.CSRF(web.UserDelete(a))))
	router.Handle("/api/user/my-token", a.Admin(a.MyToken()))
	router.Handle("/logout", a.Logout())

	router.Handle("/api/monitor/list", a.User(web.MonitorList(monitorManager.MonitorsInfo)))
	router.Handle("/api/monitor/configs", a.Admin(web.MonitorConfigs(monitorManager)))
	//router.Handle("/api/monitor/restart", a.Admin(a.CSRF(web.MonitorRestart(monitorManager))))
	//router.Handle("/api/monitor/set", a.Admin(a.CSRF(web.MonitorSet(monitorManager))))
	//router.Handle("/api/monitor/delete", a.Admin(a.CSRF(web.MonitorDelete(monitorManager))))
	router.Handle("/api/monitor/restart", a.Admin(web.MonitorRestart(monitorManager)))
	router.Handle("/api/monitor/set", a.Admin(web.MonitorSet(monitorManager)))
	router.Handle("/api/monitor/delete", a.Admin(web.MonitorDelete(monitorManager)))

	router.Handle("/api/group/configs", a.User(web.GroupConfigs(groupManager)))
	router.Handle("/api/group/set", a.Admin(a.CSRF(web.GroupSet(groupManager))))
	router.Handle("/api/group/delete", a.Admin(a.CSRF(web.GroupDelete(groupManager))))

	router.Handle("/api/recording/delete/", a.Admin(a.CSRF(web.RecordingDelete(env.RecordingsDir()))))
	router.Handle("/api/recording/thumbnail/", a.User(web.RecordingThumbnail(env.RecordingsDir())))
	router.Handle("/api/recording/video/", a.User(web.RecordingVideo(logger, env.RecordingsDir())))
	router.Handle("/api/recording/query", a.User(web.RecordingQuery(crawler, logger)))

	router.Handle("/api/log/feed", a.Admin(web.LogFeed(logger, a)))
	router.Handle("/api/log/query", a.Admin(web.LogQuery(logStore)))
	router.Handle("/api/log/sources", a.Admin(web.LogSources(logger)))

	return &App{
		WG:             wg,
		Logger:         logger,
		logStore:       logStore,
		Env:            *env,
		monitorManager: monitorManager,
		Auth:           a,
		Storage:        storageManager,
		videoServer:    videoServer,
		Templater:      t,
		Router:         router,
	}, nil
}

func (app *App) run(ctx context.Context) error {
	// Main server.
	address := ":" + strconv.Itoa(app.Env.Port)
	app.server = &http.Server{Addr: address, Handler: app.Router}

	if err := app.Logger.Start(ctx); err != nil {
		return fmt.Errorf("could not start logger: %w", err)
	}

	app.Logger.LogToWriter(ctx, os.Stdout)
	app.logStore.SaveLogs(ctx, app.Logger)
	app.logStore.PurgeLoop(ctx, app.Logger)
	time.Sleep(10 * time.Millisecond)

	if err := hooks.appRun(ctx, app); err != nil {
		return err
	}

	app.logf(log.LevelInfo, "Starting..")

	if err := app.Env.PrepareEnvironment(); err != nil {
		return fmt.Errorf("could not prepare environment: %w", err)
	}

	if err := app.videoServer.Start(ctx); err != nil {
		return fmt.Errorf("could not start video server: %w", err)
	}

	app.monitorManager.StartMonitors()

	go app.Storage.PurgeLoop(ctx, 10*time.Minute)

	app.logf(log.LevelInfo, "Serving app on port %v", app.Env.Port)
	return app.server.ListenAndServe()
}

func (app *App) logf(level log.Level, format string, a ...interface{}) {
	app.Logger.Log(log.Entry{
		Level: level,
		Src:   "app",
		Msg:   fmt.Sprintf(format, a...),
	})
}

func ExternalIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}
			return ip.String(), nil
		}
	}
	return "", errors.New("are you connected to the network?")
}
