package topic

import (
	"testing"
	"time"
)

func equalSlice(a, b []interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
func TestNode(t *testing.T) {
	root := newNode()
	root.add("a/b/+", 1, 1)
	root.add("a/b/d", 0, 2)
	root.add("#", 1, 3)
	root.add("a/+/d", 2, 4)
	root.add("a/b/#", 1, 5)
	root.add("a/+/#", 1, 6)
	root.add("+/+/#", 0, 7)
	root.add("+/b/+", 2, 8)
	root.add("aaaaa/bbbbb/ccccc/ddddd/eeeee/fffff/ggggg/hhhhh/iiiii/jjjjj", 0, 9)
	last := time.Now()
	ss := root.getSessions("a/b/d")
	t.Logf("use time:%d\n", time.Since(last))
	t.Logf("a/b/d:%#v", ss)

	last = time.Now()
	ss = root.getSessions("a/b/c")
	t.Logf("use time:%d\n", time.Since(last))
	t.Logf("a/b/c:%#v", ss)

	last = time.Now()
	ss = root.getSessions("f")
	t.Logf("use time:%d\n", time.Since(last))
	t.Logf("f:%#v", ss)

	last = time.Now()
	ss = root.getSessions("g/h")
	t.Logf("use time:%d\n", time.Since(last))
	t.Logf("g/h:%#v", ss)

	last = time.Now()
	ss = root.getSessions("a/2/d")
	t.Logf("use time:%d\n", time.Since(last))
	t.Logf("a/2/d:%#v", ss)

}

func BenchmarkNodeAdd(b *testing.B) {
	root := newNode()
	for i := 0; i < b.N; i++ {
		root.add("aaaaa/bbbbb/ccccc/ddddd/eeeee/fffff/ggggg/hhhhh/iiiii/jjjjj", 0, i)
	}

}
func BenchmarkNodeGetSessions(b *testing.B) {
	b.StopTimer()
	root := newNode()
	for i := 0; i < b.N; i++ {
		root.add("aaaaa/bbbbb/ccccc/ddddd/eeeee/fffff/ggggg/hhhhh/iiiii/jjjjj", 0, i)
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		root.getSessions("aaaaa/bbbbb/ccccc/ddddd/eeeee/fffff/ggggg/hhhhh/iiiii/jjjjj")
	}
}

func TestIsTopicMatch(t *testing.T) {
	root := newNode()

	topic := "a/b/c/d/e"
	sub := "a/b/c/d/e"
	if root.IsTopicMatch(topic, sub) != true {
		t.Fatal("a/b/c/d/e not match a/b/c/d/e")
	}
	sub = "a/b/+/d/e"
	if root.IsTopicMatch(topic, sub) != true {
		t.Fatal("a/b/c/d/e not match a/b/+/d/e")
	}
	sub = "a/#"
	if root.IsTopicMatch(topic, sub) != true {
		t.Fatal("a/b/c/d/e not match a/#")
	}
	sub = "a/b/+/#"
	if root.IsTopicMatch(topic, sub) != true {
		t.Fatal("a/b/c/d/e not match a/b/+/#")
	}
	topic = "a/b/c/d/e/d"
	sub = "a/b/c/d/e"
	if root.IsTopicMatch(topic, sub) == true {
		t.Fatal("a/b/c/d/e/d not match a/b/c/d/e")
	}
	topic = "a/b/c/d"
	sub = "a/b/c/d/e"
	if root.IsTopicMatch(topic, sub) == true {
		t.Fatal("a/b/c/d not match a/b/c/d/e")
	}
	topic = "a/b/c/d/"
	sub = "a/b/c/d/+"
	if root.IsTopicMatch(topic, sub) == false {
		t.Fatal("a/b/c/d/ not match a/b/c/d/+")
	}
	topic = "a/b/c/d"
	sub = "a/b/c/d/+"
	if root.IsTopicMatch(topic, sub) == false {
		t.Fatal("a/b/c/d not match a/b/c/d/+")
	}
	topic = "a/b/c/d/"
	sub = "a/b/c/d/#"
	if root.IsTopicMatch(topic, sub) == false {
		t.Fatal("a/b/c/d/ not match a/b/c/d/+")
	}
	topic = "a/b/c/d"
	sub = "a/b/c/d/#"
	if root.IsTopicMatch(topic, sub) == false {
		t.Fatal("a/b/c/d/ not match a/b/c/d/+")
	}
}
