package recipe

type StorageProfile string

const (
	StorageKeeps3d       StorageProfile = "keeps_3d"
	StorageKeeps3dReheat StorageProfile = "keeps_3d_reheat"
	StorageAssembleFresh StorageProfile = "assemble_fresh"
	StorageEatSameDay    StorageProfile = "eat_same_day"
)
