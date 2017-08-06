package topic

import (
	"testing"
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
func checkSessions(t *testing.T, m SessoinQosMap, s []int) {
	if len(m) != len(s) {
		t.FailNow()
	}
	for _, c := range s {
		if _, ok := m[c]; !ok {
			t.FailNow()
		}

	}
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

	ss := root.getSessions("a/b/d")

	ss = root.getSessions("a/b/c")
	checkSessions(t, ss, []int{1, 3, 5, 6, 7, 8})

	ss = root.getSessions("f")
	checkSessions(t, ss, []int{3})
	ss = root.getSessions("g/h")
	checkSessions(t, ss, []int{3})

	ss = root.getSessions("a/2/d")
	checkSessions(t, ss, []int{3, 4, 6, 7})

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
