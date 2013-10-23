package main

// adapted from http://rogpeppe.wordpress.com/2010/02/10/unlimited-buffering-with-low-overhead/

import (
	"container/list"
)

type ElementMatcher func(a, b GithubJson) bool

func CoalescingBufferList(out chan<- GithubJson, matcher ElementMatcher) chan<- GithubJson {
	in := make(chan GithubJson, 100)
	go func() {
		buf := list.New()
		for {
			outc := out
			var v GithubJson
			n := buf.Len()
			if n == 0 {
				// buffer empty: don't try to send on output
				if in == nil {
					close(out)
					return
				}
				outc = nil
			} else {
				v = buf.Front().Value.(GithubJson)
			}
			select {
			case e, ok := <-in:
				if !ok {
					in = nil
				} else {
					pushUnlessContains(buf, e, matcher)
				}
			case outc <- v:
				buf.Remove(buf.Front())
			}
		}
	}()
	return in
}

func pushUnlessContains(l *list.List, e GithubJson, matcher ElementMatcher) {
	if !bufferListContains(l, e, matcher) {
		l.PushBack(e)
	}
}

func bufferListContains(l *list.List, potentialE GithubJson, matcher ElementMatcher) bool {
	for e := l.Front(); e != nil; e = e.Next() {
		// do something with e.Value
		if matcher(potentialE, e.Value.(GithubJson)) {
			return true
		}
	}

	return false
}
