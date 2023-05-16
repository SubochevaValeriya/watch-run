package worker

import (
	"context"
	"fmt"
	"github.com/radovskyb/watcher"
	"github.com/sirupsen/logrus"
	"log"
	"os"
	"strings"
	"time"
	"watchAndRun/internal/app/watch-and-run/model"
	"watchAndRun/internal/app/watch-and-run/repository"
)

type ApiService struct {
	repo repository.Repository
}

func newApiService(repo repository.Repository) *ApiService {
	return &ApiService{repo: repo}
}

type Worker interface {
	Watch(ctx context.Context, dir model.Directory, changeCheckFrequency time.Duration)
}

type Service struct {
	Worker
}

func NewService(repos *repository.Repository) *Service {
	return &Service{newApiService(*repos)}
}

type Watcher interface {
	WatchChanges()
}

func (s *ApiService) Watch(ctx context.Context, dir model.Directory, changeCheckFrequency time.Duration) {
	w := watcher.New()

	// Only notify create, write and remove events.
	w.FilterOps(watcher.Create, watcher.Write, watcher.Remove)

	// Watch only differences in files (not hidden):
	w.AddFilterHook(filterOnlyFiles)

	// Watch folder recursively for changes.
	if err := w.AddRecursive(dir.Path); err != nil {
		log.Fatalln(err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("Canceling")
				w.Close()
				return
			case event := <-w.Event:
			checkRegexp:
				for _, regexpIncl := range dir.IncludeRegexp {
					if regexpIncl.MatchString(event.Name()) {
						for _, regexpExcl := range dir.ExcludeRegexp {
							if regexpExcl.MatchString(event.Name()) {
								break checkRegexp
							}
						}
						fmt.Println(event)
						currentEvent := model.Event{
							Id:        0,
							Path:      dir.Path,
							FileName:  event.Name(),
							EventType: event.Op.String(),
							Time:      time.Now(),
						}

						eventId, err := s.repo.AddEvent(currentEvent)
						if err != nil {
							logrus.Errorf("Can't save event data: %s", err)
						}
						for _, command := range dir.Commands {
							launch := model.Launch{
								Id:        0,
								Command:   command,
								StartTime: time.Now(),
								EndTime:   time.Time{},
								Result:    "",
								EventId:   int(eventId),
							}

							fmt.Println(command, dir.LogFile)
							err := executeCommand(command, dir.LogFile)
							launch.EndTime = time.Now()
							if err == nil {
								launch.Result = "success"
								err = s.repo.AddLaunch(launch)
								if err != nil {
									logrus.Errorf("Can't save launch data: %v", err)
								}
							} else {
								launch.Result = "failure"
								err = s.repo.AddLaunch(launch)
								if err != nil {
									logrus.Errorf("Can't save launch data: %v", err)
								}
								break
							}
						}
						fmt.Println("found", dir.Path)
					}
				}
			case err := <-w.Error:
				log.Fatalln(err)
			case <-w.Closed:
				return
			}
		}
	}()

	// Wait after watcher started.
	go func() {
		w.Wait()
	}()

	// Start the watching process - it'll check for changes every 100ms.
	if err := w.Start(changeCheckFrequency); err != nil {
		log.Fatalln(err)
	}

}

func filterOnlyFiles(info os.FileInfo, fullPath string) error {
	if info.IsDir() {
		return watcher.ErrSkip
	}

	if strings.Index(fullPath, "\\.") != -1 {
		return watcher.ErrSkip
	}

	return nil
}
