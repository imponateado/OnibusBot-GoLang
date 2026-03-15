package domain

type SubscriptionRepository interface {
	Save(subs []UserSubscription) error
	Load() ([]UserSubscription, error)
}

type GroupRepository interface {
	Load() ([]BusGroup, error)
}
