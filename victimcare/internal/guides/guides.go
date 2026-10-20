// Package guides holds the first-aid content shown in the First aid screen.
//
// The steps are written in plain, imperative language so they can be followed
// while someone is stressed or injured.
package guides

// Guide is a single first-aid topic.
type Guide struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Summary  string   `json:"summary"`
	Steps    []string `json:"steps"`
	CallHelp string   `json:"callHelp"`
}

// All returns the complete guide list in display order.
func All() []Guide {
	return []Guide{
		{
			ID:      "bleeding",
			Title:   "Heavy bleeding",
			Summary: "Stop the blood loss first.",
			Steps: []string{
				"Press firmly on the wound with a clean cloth or your hand.",
				"Keep pressing without lifting the cloth to check.",
				"Raise the injured limb above the level of the heart if you can.",
				"If blood soaks through, add another cloth on top and keep pressing.",
				"Call for help now and stay with the person.",
			},
			CallHelp: "Call the emergency number as soon as you can.",
		},
		{
			ID:      "burns",
			Title:   "Burns",
			Summary: "Cool the burn, keep it clean.",
			Steps: []string{
				"Move away from the source of the burn.",
				"Cool the burn under cool running water for 20 minutes.",
				"Remove rings, watches and tight clothing near the burn.",
				"Cover loosely with cling film or a clean non-stick dressing.",
				"Do not use ice, butter, oil or toothpaste.",
			},
			CallHelp: "Get medical help for large or deep burns.",
		},
		{
			ID:      "choking",
			Title:   "Choking",
			Summary: "Help the person clear their airway.",
			Steps: []string{
				"Ask them to cough forcefully. If they can cough, keep encouraging them.",
				"If they cannot speak, cough or breathe, give 5 firm back blows.",
				"Give 5 abdominal thrusts: place a fist above the navel and pull sharply inwards.",
				"Repeat 5 back blows and 5 thrusts until the object comes out or help arrives.",
				"Call the emergency number if the person becomes unresponsive.",
			},
			CallHelp: "Call the emergency number if the object does not come out.",
		},
		{
			ID:      "breathing",
			Title:   "Not breathing",
			Summary: "Start CPR and get help.",
			Steps: []string{
				"Call the emergency number and ask for an ambulance and a defibrillator.",
				"Place the person on their back on firm ground.",
				"Push hard and fast in the centre of the chest, about twice per second.",
				"Push at least 5 cm deep for an adult and allow the chest to come back up.",
				"Keep going until help takes over.",
			},
			CallHelp: "Call the emergency number first if the person is not breathing.",
		},
		{
			ID:      "bone",
			Title:   "Broken bone",
			Summary: "Keep the person still and comfortable.",
			Steps: []string{
				"Keep the person still and support the injured part in the position found.",
				"Do not try to straighten the limb or push the bone back.",
				"Pad around the injury with rolled-up cloth or clothing.",
				"Apply a cold pack wrapped in cloth for up to 20 minutes to reduce swelling.",
				"Get medical help.",
			},
			CallHelp: "Call the emergency number if the bone is open or the person cannot move.",
		},
		{
			ID:      "shock",
			Title:   "Shock",
			Summary: "Keep the person warm and calm.",
			Steps: []string{
				"Lay the person down and raise their legs if they are not badly hurt.",
				"Cover them with a coat or blanket, but do not use an electric blanket.",
				"Loosen tight clothing around the neck, chest and waist.",
				"Keep them calm and do not give food or drink.",
				"Monitor their breathing and call for help.",
			},
			CallHelp: "Call the emergency number if the person is pale, cold or confused.",
		},
	}
}

// ByID returns a single guide.
func ByID(id string) (Guide, bool) {
	for _, g := range All() {
		if g.ID == id {
			return g, true
		}
	}
	return Guide{}, false
}
