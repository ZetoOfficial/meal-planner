package planning

type PlanID int64

type Plan struct {
	ID   PlanID
	Days int
}

func (p *Plan) AttachQuality(_ any) {}
