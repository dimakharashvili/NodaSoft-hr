package main

import (
	"fmt"
	"sync"
	"time"
)

// ЗАДАНИЕ:
// * сделать из плохого кода хороший;
// * важно сохранить логику появления ошибочных тасков;
// * сделать правильную мультипоточность обработки заданий.
// Обновленный код отправить через merge-request.

// приложение эмулирует получение и обработку тасков, пытается и получать и обрабатывать в многопоточном режиме
// В конце должно выводить успешные таски и ошибки выполнены остальных тасков

// A Ttype represents a meaninglessness of our life

type Task struct { // экспорт?
	id       int
	created  time.Time // время создания
	finished time.Time // время выполнения
	execErr  error
}

func (t *Task) String() string {
	return fmt.Sprintf(
		"id=%d, created=%v, finished=%v",
		t.id,
		t.created.Format(time.RFC3339Nano),
		t.finished.Format(time.RFC3339Nano))
}

func tasksProducer(exit <-chan struct{}) (out chan *Task) {
	out = make(chan *Task, 10)
	id := 0 // id был не уникальным, для простоты заменим на счетчик

	go func() {
		for {
			select {
			case <-exit:
				close(out)
				return
			default:
				var execErr error
				created := time.Now()
				if time.Now().Nanosecond()%2 > 0 { // вот такое условие появления ошибочных тасков
					execErr = fmt.Errorf("some error occured")
				}
				id++
				out <- &Task{created: created, id: id, execErr: execErr} // передаем таск на выполнение
			}

		}
	}()
	return out
}

// т.к. обработка таски ресурсоемка, то имеет смысл использовать пулл обработчиков
func tasksHandler(in <-chan *Task, exit <-chan struct{}, poolSize int) (out chan *Task) {
	out = make(chan *Task, 10)

	taskWorker := func(t *Task) *Task {
		res := &Task{id: t.id, created: t.created}
		if t.execErr != nil {
			res.execErr = fmt.Errorf("task is already broken: %w", t.execErr)
		} else if t.created.Before(time.Now().Add(-500 * time.Millisecond)) {
			res.execErr = fmt.Errorf("something went wrong")
		}
		res.finished = time.Now()

		time.Sleep(time.Millisecond * 200)
		return res
	}

	wg := &sync.WaitGroup{}
	go func() {
		for i := 0; i < poolSize; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					// получение тасков
					case t, open := <-in: // проверяем на всякий случай, если канал продьюсера создавали не мы
						if !open {
							return
						}
						t = taskWorker(t)
						out <- t
					case <-exit:
						return
					}
				}
			}()
		}
		wg.Wait()
		close(out)
	}()
	return out
}

func tasksSorter(in <-chan *Task, exit <-chan struct{}) (outTask chan *Task, outErr chan error) {
	outErr = make(chan error, 10)
	outTask = make(chan *Task, 10)

	wg := &sync.WaitGroup{}
	go func() {
		defer func() {
			wg.Wait()
			close(outTask)
			close(outErr)
		}()
		for {
			select {
			// получение тасков
			case t, open := <-in:
				if !open {
					return
				}
				wg.Add(1)
				go func(t *Task) {
					defer wg.Done()
					if t.execErr != nil {
						outErr <- fmt.Errorf("task id: %d time: %s, error: %w", t.id, t.created.Format(time.RFC3339Nano), t.execErr) // можно сделать свой тип error
					} else {
						outTask <- t
					}
				}(t)
			case <-exit:
				return
			}
		}
	}()
	return outTask, outErr
}

func main() {

	exit := make(chan struct{})

	// пайплайн для обработки тасок
	producedTasks := tasksProducer(exit)
	unsortedTasks := tasksHandler(producedTasks, exit, 5)
	doneTasks, failedTasks := tasksSorter(unsortedTasks, exit)

	result := make(map[int]*Task) // сделал последовательный доступ, либо нужен sync map (нужна ли map в целом?)
	errors := []error{}           // слайс так же модифицируется последовательно, либо обернуть мьютексом

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case t := <-doneTasks:
				result[t.id] = t
			case err := <-failedTasks:
				errors = append(errors, err)
			case <-exit:
				return
			}
		}
	}()

	time.Sleep(time.Second * 3)

	close(exit)
	wg.Wait() //подождем, чтобы не было датарейса на чтении

	fmt.Println("Errors:") // to standart output, not standart error?
	for _, err := range errors {
		fmt.Println(err)
	}

	fmt.Println("Done tasks:")
	for _, t := range result {
		tmp := t
		fmt.Println(tmp)
	}
}
