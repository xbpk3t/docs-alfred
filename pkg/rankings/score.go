package rankings

// ---------- 内容热度 ----------

// ArticleHeatCalc 用于内容热度排行。
// 公式：views*ViewWeight + likes*LikeWeight + comments*CommentWeight。
// 时间衰减由 DecayStrategy 在外部处理，这里只做加权聚合。
type ArticleHeatCalc struct {
	ViewWeight    float64
	LikeWeight    float64
	CommentWeight float64
}

func NewArticleHeatCalc() *ArticleHeatCalc {
	return &ArticleHeatCalc{
		ViewWeight:    1.0,
		LikeWeight:    3.0,
		CommentWeight: 5.0,
	}
}

func (c *ArticleHeatCalc) Calculate(item Item) (float64, error) {
	views := item.Fields["views"]
	likes := item.Fields["likes"]
	comments := item.Fields["comments"]

	score := views*c.ViewWeight + likes*c.LikeWeight + comments*c.CommentWeight

	if err := validateScore(score, "article_heat"); err != nil {
		return 0, err
	}
	return score, nil
}
