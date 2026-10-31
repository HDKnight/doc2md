package pipeline

// joinHeadingFragments：折行已在 soft-wrap 里按栏右缘拼完。这里只把拼成多句的标题降回正文。
func joinHeadingFragments(segs []seg) []seg {
	return demoteExpandedHeadings(segs)
}

// demoteExpandedHeadings：折行拼完后若已成多句论述 → 降为正文。
func demoteExpandedHeadings(segs []seg) []seg {
	for i := range segs {
		if segs[i].kind != "heading" {
			continue
		}
		if segs[i].series != "" {
			continue
		}
		if expandedProseBlock(segs[i].text) {
			segs[i].kind = "para"
			segs[i].lvl = 0
		}
	}
	return segs
}
