/*
 * Copyright (c) 2017-2018 Miguel Ángel Ortuño.
 * See the COPYING file for more information.
 */

package concurrent_test

import (
	"testing"
	"time"

	"github.com/ortuman/jackal/concurrent"
	"github.com/stretchr/testify/assert"
)

func TestAsync(t *testing.T) {
	queue := concurrent.OperationQueue{}
	var v int
	for i := 0; i < 128; i++ {
		queue.Async(func() {
			v++
		})
	}
	for i := 0; i < 128; i++ {
		queue.Async(func() {
			v++
		})
	}
	time.Sleep(time.Second)
	assert.Equal(t, v, 256)
}

func TestSync(t *testing.T) {
	queue := concurrent.OperationQueue{}
	flag := false
	queue.Sync(func() {
		flag = true
	})
	assert.Equal(t, flag, true)
}