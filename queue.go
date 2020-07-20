// Copyright (C) 2020  Raziman

package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path"
	"time"

	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
)

type Queue struct {
	*tview.List
	SavedQueuePath string
	Items          []*AudioFile
	IsLoop         bool
}

// highlight the next item in the queue
func (q *Queue) next() {
	currIndex := q.GetCurrentItem()
	idx := currIndex + 1
	if currIndex == q.GetItemCount()-1 {
		idx = 0
	}
	q.SetCurrentItem(idx)
}

// highlight the previous item in the queue
func (q *Queue) prev() {
	currIndex := q.GetCurrentItem()
	q.SetCurrentItem(currIndex - 1)
}

// usually used with GetCurrentItem which can return -1 if
// no item highlighted
func (q *Queue) DeleteItem(index int) (*AudioFile, error) {

	if index > len(q.Items)-1 {
		return nil, errors.New("Index out of range")
	}

	// deleted audio file
	var dAudio *AudioFile

	if index != -1 {
		q.RemoveItem(index)

		var nItems []*AudioFile

		for i, v := range q.Items {

			if i == index {
				dAudio = v
				continue
			}

			nItems = append(nItems, v)
		}

		q.Items = nItems
		q.UpdateTitle()

	}

	return dAudio, nil
}

// Update queue title which shows number of items and total length
func (q *Queue) UpdateTitle() {

	var totalLength time.Duration

	for _, v := range q.Items {
		totalLength += v.Length
	}

	fmtTime := fmtDuration(totalLength)

	q.SetTitle(fmt.Sprintf("┤ Queue ├──┤%d|%s├", len(q.Items), fmtTime))

}

// Add item to the front of the queue
func (q *Queue) PushFront(audioFile *AudioFile) {

	q.Items = append([]*AudioFile{audioFile}, q.Items...)

	songLength := audioFile.Length

	queueItemView := fmt.Sprintf(
		"[ %s ] %s", fmtDuration(songLength), GetName(audioFile.Name),
	)

	q.InsertItem(0, queueItemView, audioFile.Path, 0, nil)
	q.UpdateTitle()
}

// gets the first item and remove it from the queue
// app.Draw() must be called after calling this function
func (q *Queue) Dequeue() (*AudioFile, error) {

	if q.GetItemCount() == 0 {
		return nil, errors.New("Empty list\n")
	}

	first := q.Items[0]
	q.DeleteItem(0)
	q.UpdateTitle()

	return first, nil
}

// Add item to the list and returns the length of the queue
func (q *Queue) Enqueue(audioFile *AudioFile) (int, error) {

	if !gomu.Player.IsRunning && "false" == os.Getenv("TEST") {

		gomu.Player.IsRunning = true

		go func() {

			gomu.Player.Run(audioFile)

		}()

		return q.GetItemCount(), nil

	}

	q.Items = append(q.Items, audioFile)
	songLength, err := GetLength(audioFile.Path)

	if err != nil {
		return 0, WrapError("Enqueue", err)
	}

	queueItemView := fmt.Sprintf(
		"[ %s ] %s", fmtDuration(songLength), GetName(audioFile.Name),
	)
	q.AddItem(queueItemView, audioFile.Path, 0, nil)
	q.UpdateTitle()

	return q.GetItemCount(), nil
}

// GetItems is used to get the secondary text
// which is used to store the path of the audio file
// this is for the sake of convenience
func (q *Queue) GetItems() []string {

	items := []string{}

	for i := 0; i < q.GetItemCount(); i++ {

		_, second := q.GetItemText(i)

		items = append(items, second)
	}

	return items
}

// Save the current queue in a csv file
func (q *Queue) SaveQueue() error {

	songPaths := q.GetItems()
	var content string

	for _, songPath := range songPaths {
		hashed := Sha1Hex(GetName(songPath))
		content += hashed + "\n"
	}

	cachePath := expandTilde(q.SavedQueuePath)
	err := ioutil.WriteFile(cachePath, []byte(content), 0644)

	if err != nil {
		return WrapError("SaveQueue", err)
	}

	return nil

}

// Clears current queue
func (q *Queue) ClearQueue() {

	q.Items = []*AudioFile{}
	q.Clear()
	q.UpdateTitle()

}

// Loads previously saved list
func (q *Queue) LoadQueue() error {

	songs, err := q.GetSavedQueue()

	if err != nil {
		return WrapError("LoadQueue", err)
	}

	for _, v := range songs {

		audioFile := gomu.Playlist.FindAudioFile(v)

		if audioFile != nil {
			q.Enqueue(audioFile)
		}
	}

	return nil
}

// Get saved queue, if not exist, create it
func (q *Queue) GetSavedQueue() ([]string, error) {

	fnName := "GetSavedQueue"

	queuePath := expandTilde(q.SavedQueuePath)

	if _, err := os.Stat(queuePath); os.IsNotExist(err) {

		dir, _ := path.Split(queuePath)

		err := os.MkdirAll(dir, 0744)
		if err != nil {
			return nil, WrapError(fnName, err)
		}

		_, err = os.Create(queuePath)
		if err != nil {
			return nil, WrapError(fnName, err)
		}

		return []string{}, nil

	}

	f, err := os.Open(queuePath)
	if err != nil {
		return nil, WrapError(fnName, err)
	}

	defer f.Close()

	records := []string{}
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		records = append(records, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, WrapError(fnName, err)
	}

	return records, nil
}

func (q *Queue) Help() []string {

	return []string{
		"j      down",
		"k      up",
		"l      play selected song",
		"d      remove from queue",
		"D      clear queue",
		"z      toggle loop",
		"s      shuffle",
	}

}

func (q *Queue) Shuffle() {

	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(q.Items), func(i, j int) { 
		q.Items[i], q.Items[j] = q.Items[j], q.Items[i] 
	})

	q.Clear()

	for _, v := range q.Items {
		audioLen, err := GetLength(v.Path)
		log.Println(err)

		queueText := fmt.Sprintf("[ %s ] %s", fmtDuration(audioLen), v.Name)
		q.AddItem(queueText, v.Path, 0, nil)
	}

	q.UpdateTitle()

}

// Initiliaze new queue with default values
func NewQueue() *Queue {

	list := tview.NewList().
		ShowSecondaryText(false)

	queue := &Queue{
		List:           list,
		SavedQueuePath: "~/.local/share/gomu/queue.cache",
	}

	queue.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {

		switch e.Rune() {
		case 'j':
			queue.next()
		case 'k':
			queue.prev()
		case 'd':
			queue.DeleteItem(queue.GetCurrentItem())
		case 'D':
			queue.ClearQueue()
		case 'l':
			a, err := queue.DeleteItem(queue.GetCurrentItem())
			if err != nil {
				log.Println(err)
			}

			queue.PushFront(a)
			gomu.Player.Skip()
		case 'z':
			isLoop := gomu.Player.ToggleLoop()
			var msg string

			if isLoop {
				msg = "on"
			} else {
				msg = "off"
			}

			timedPopup("Loop", msg, getPopupTimeout(), 30, 5)
		case 's':
			queue.Shuffle()
		}

		return nil
	})

	queue.UpdateTitle()
	queue.SetBorder(true).SetTitleAlign(tview.AlignLeft)
	queue.
		SetSelectedBackgroundColor(tcell.ColorDarkCyan).
		SetSelectedTextColor(tcell.ColorWhite).
		SetHighlightFullLine(true)

	return queue

}

func Sha1Hex(input string) string {
	h := sha1.New()
	h.Write([]byte(input))
	return hex.EncodeToString(h.Sum(nil))
}
