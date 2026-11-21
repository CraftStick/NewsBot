package news

import "testing"

func TestMiniCore(t *testing.T) {
	for in, want := range map[string]bool{
		"В Windows 11 нашли способ обойти экран блокировки. Уязвимость": false,
		"Роскомнадзор начал замедлять сервис":                           true,
		"Хакеры взломали Lenovo":                                        true,
		"Утечка данных клиентов":                                        true,
	} {
		if miniCore(in, "") != want {
			t.Errorf("%q: want %v", in, want)
		}
	}
}
