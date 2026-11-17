package news

import "testing"

func TestTGPhotoRE(t *testing.T) {
	with := `<a class="tgme_widget_message_photo_wrap 1_2" href="x" style="width:800px;background-image:url('https://cdn4.telesco.pe/file/a.jpg')">`
	if m := tgPhotoRE.FindStringSubmatch(with); m == nil || m[1] != "https://cdn4.telesco.pe/file/a.jpg" {
		t.Fatalf("фото не найдено: %v", m)
	}
	if tgPhotoRE.MatchString(`<i style="background-image:url('//telegram.org/img/emoji/40/E29C88.png')">`) {
		t.Fatal("эмодзи принят за фото")
	}
}
