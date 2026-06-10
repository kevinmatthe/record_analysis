package textclean

import "testing"

func TestCleanMessageTextDropsQuotedWeChatXML(t *testing.T) {
	input := `看啦
> KevinMatt：<?xml version="1.0"?><msg><appmsg appid="wxcb8d"><title>加藤去贵阳vlog</title><des>UP主：加藤在中国 播放：4.1万</des><type>4</type><url>https://b23.tv/test</url><nickname>null</nickname><msgid>123</msgid><![CDATA[noise]]></appmsg></msg>`

	got := CleanMessageText(input)
	if got != "看啦" {
		t.Fatalf("cleaned = %q", got)
	}
}

func TestCleanMessageTextExtractsHumanShareTitle(t *testing.T) {
	input := `<msg><appmsg appid="wxcb8d"><title>咪：我已出舱，感觉良好</title><des>UP主：新派世角 播放：12.5万</des><type>4</type><nickname>null</nickname><msgid>123</msgid></appmsg></msg>`

	got := CleanMessageText(input)
	if got != "咪：我已出舱，感觉良好 UP主：新派世角 播放：12.5万" {
		t.Fatalf("cleaned = %q", got)
	}
}

func TestIsNaturalMessageTextRejectsWeChatStructureOnly(t *testing.T) {
	if IsNaturalMessageText(`nickname type appid msg cdata null msgid`) {
		t.Fatal("expected structural noise to be rejected")
	}
}
