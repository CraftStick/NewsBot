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

func TestMiniCoreFromLogs(t *testing.T) {
	for _, title := range []string{
		"Почему из соглашения Яндекс Браузера пропала анонимность: ответ компании",
		"Сканер нашел уязвимость, но бизнесу всё равно нужен пентест",
	} {
		if !miniCore(title, "") {
			t.Errorf("должно проходить: %s", title)
		}
	}
	for _, title := range []string{
		"Теперь Wishmaster можно сделать самому",
		"Softline: сканер нашел уязвимость или почему бизнесу все равно нужен пентест",
		"ГК Softline: ИИ-агенты могут атаковать системы",
		"Складок нет, но вы держитесь",
		"Anthropic нашла способы предотвратить побег ИИ из песочниц",
		"Microsoft объявила «красный код» после масштабного сбоя Microsoft 365",
	} {
		if miniCore(title, "") {
			t.Errorf("не должно проходить: %s", title)
		}
	}
}
