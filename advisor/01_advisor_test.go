// /home/krylon/go/src/ticker/advisor/01_advisor_test.go
// -*- mode: go; coding: utf-8; -*-
// Created on 27. 05. 2021 by Benjamin Walkenhorst
// (c) 2021 Benjamin Walkenhorst
// Time-stamp: <2024-10-22 17:15:06 krylon>

package advisor

import (
	"testing"

	"github.com/blicero/badnews/model"
)

var ad *Advisor

func TestInitAdvisor(t *testing.T) {
	var err error

	if ad, err = NewAdvisor(); err != nil {
		ad = nil
		t.Fatalf("Cannot create new Advisor: %s",
			err.Error())
	}
}

func TestPanic(t *testing.T) {
	if ad == nil {
		t.SkipNow()
	}

	defer func() {
		if x := recover(); x != nil {
			ad = nil
			t.Fatalf("Someone panicked: %s", x)
		}
	}()

	var item = model.Item{
		ID:       1,
		FeedID:   1,
		Headline: `Anti-government activist Ammon Bundy mocked for seeking to run Idaho government -- despite being banned from Capitol grounds`,
		Description: `
<img src="https://www.rawstory.com/media-library/eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpbWFnZSI6Imh0dHBzOi8vYXNzZXRzLnJibC5tcy8yNjQzNDc1NC9vcmlnaW4uanBnIiwiZXhwaXJlc19hdCI6MTY2MTU0OTc3Nn0.Dtbwg5qOp0HhWS5eFkHUkIIGLK8z4T3a58Lf3SUvJCg/image.jpg?width=1200&coordinates=0%2C0%2C0%2C61&height=600"/><br/><br/><p>Anti-government activist Ammon Bundy is now begging for a government job that would hand him the keys to the government he hates. According to <a href="https://www.ktvb.com/article/news/politics/ammon-bundy-run-idaho-governor/277-aa8a55c1-3998-42ba-958e-b7809547b6bc" rel="noopener noreferrer" target="_blank">filed papers Bundy intends run for governor </a>in Idaho. The problem, however, is that Bundy isn't even registered to vote in Idaho and he's not legally allowed to go on Capitol grounds. It could make it difficult to take over the state.</p><p><a href="https://www.nbcnews.com/news/us-news/noted-anti-government-activist-ammon-bundy-running-governor-idaho-n1268317" target="_blank">Speaking to NBC News on Monday</a>, Bundy said that it isn't official but he's building out a team. </p><p>"The people of Idaho are very freedom-minded," Bundy said. "I had never desired (to run for office), but I knew as early as 2017 that I would run for governor of Idaho."</p><p>It's unclear why he isn't registered to vote in the state after knowing for four years that he'd run. </p><p><span></span>Republican Gov. Brad Little (ID) has faced unsuccessful attempts to recall him by activists like Bundy. </p><p>Bundy is the son of Cliven Bundy, who refused to pay grazing fees to graze his cattle on federal lands in Nevada. The Bundys refused to pay their bills for 21 years and when the government came to collect the family it turned into an armed standoff. Bundy continues to use the federal land and refuses to pay for it. </p><p>The announcement sent many to mock Bundy with hilarity: </p><p><br/></p><blockquote class="twitter-tweet">
<p dir="ltr" lang="en">Oddly grateful that Ammon Bundy filed to run for Idaho Governor. The Bat Shit Crazy vote is now split 🤣</p>— jazztater (@jazztater)
          <a href="https://twitter.com/jazztater/status/1395885699472060417">1621640059.0</a>
</blockquote>
<script async="" charset="utf-8" src="https://platform.twitter.com/widgets.js"></script><p><br/></p><blockquote class="twitter-tweet">
<p dir="ltr" lang="en">WTF News:

Right-wing agitator Ammon Bundy is banned from the Idaho Capitol and its grounds, but he's running for g… https://t.co/MTHB4Du3oA</p>— Bob Lawrence 🏴‍☠️🏴󠁧󠁢󠁳󠁣󠁴󠁿 Obama is #1 🇬🇧 (@TrumpluvsObama)
          <a href="https://twitter.com/TrumpluvsObama/status/1396085431591051267">1621687679.0</a>
</blockquote>
<script async="" charset="utf-8" src="https://platform.twitter.com/widgets.js"></script><p><br/></p><blockquote class="twitter-tweet">
<p dir="ltr" lang="en">Ammon Bundy, the guy who orchestrated a standoff at a bird sanctuary here in Oregon, files paperwork to run for Ida… https://t.co/NTMPUrIbWt</p>— Nick Walden Poublon (@NWPinPDX)
          <a href="https://twitter.com/NWPinPDX/status/1396132643691892742">1621698935.0</a>
</blockquote>
<script async="" charset="utf-8" src="https://platform.twitter.com/widgets.js"></script><p><br/></p><blockquote class="twitter-tweet">
<p dir="ltr" lang="en">If you vote for Ammon Bundy to be the governor of Idaho we can't be friends.</p>— Amberly (@TheAmberlyJB)
          <a href="https://twitter.com/TheAmberlyJB/status/1395937090588667910">1621652312.0</a>
</blockquote>
<script async="" charset="utf-8" src="https://platform.twitter.com/widgets.js"></script><p><br/></p><blockquote class="twitter-tweet">
<p dir="ltr" lang="en">Ammon Bundy Hates The Government So Much He Wants To Be Governor Of Idaho https://t.co/bloReVf5et</p>— Wonkette (@Wonkette)
          <a href="https://twitter.com/Wonkette/status/1396181509242081280">1621710586.0</a>
</blockquote>
<script async="" charset="utf-8" src="https://platform.twitter.com/widgets.js"></script><p><br/></p><blockquote class="twitter-tweet">
<p dir="ltr" lang="en">Seems like a good time to re-up this video of me heckling Ammon Bundy as his ass gets arrested. https://t.co/Vsbw1Zfh3X</p>— Emily Walton, Mask Wearer (@Walton_Emily)
          <a href="https://twitter.com/Walton_Emily/status/1395888029999403008">1621640615.0</a>
</blockquote>
<script async="" charset="utf-8" src="https://platform.twitter.com/widgets.js"></script><p><br/></p><blockquote class="twitter-tweet">
<p dir="ltr" lang="en">Can you be governor if you're in jail? Asking for a frie—Ammon Bundy. https://t.co/HzpTqO8fOz</p>— Idaho Democratic Party (@IdahoDems)
          <a href="https://twitter.com/IdahoDems/status/1395869503506894852">1621636198.0</a>
</blockquote>
<script async="" charset="utf-8" src="https://platform.twitter.com/widgets.js"></script>
`,
	}

	var lng, txt string

	lng, txt = ad.getLanguage(&item)

	t.Logf("Test Item has language %s, %d characters long",
		lng,
		len(txt))
} // func TestPanic(t *testing.T)
