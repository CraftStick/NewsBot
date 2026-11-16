package digest

import (
	"fmt"
	"strings"
	"testing"
)

func TestValidNewsBlocksKeepsGoodDropsBad(t *testing.T) {
	var b strings.Builder
	b.WriteString("Вот 6 главных новостей недели:\n\n")
	for i := 1; i <= 5; i++ {
		fmt.Fprintf(&b, "<b>%d. Заголовок номер %d</b>\nПервое предложение достаточно длинное для проверки. Второе тоже длинное предложение.\n\n", i, i)
	}
	b.WriteString("<b>6. Короткий</b>\nКоротко.")
	if got := len(ValidNewsBlocks(b.String())); got != 5 {
		t.Fatalf("ожидали 5 валидных блоков, получили %d", got)
	}
}
