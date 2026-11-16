package news

import "testing"

func TestFilterOnTopic(t *testing.T) {
	on := Article{Title: "РКН заблокировал VPN", Relevance: relevanceTopic}
	off := Article{Title: "Девелоперы используют ИИ в продажах", Relevance: relevanceEntity}
	if got := FilterOnTopic([]Article{on, off}, 1); len(got) != 1 || got[0].Title != on.Title {
		t.Fatalf("got %v", got)
	}
	if got := FilterOnTopic([]Article{on, off}, 5); len(got) != 2 {
		t.Fatalf("мало тематических — должен вернуться весь список: %v", got)
	}
}
