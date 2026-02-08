package tts

// VoiceForSpeaker returns the appropriate voice from the map for a given speaker name.
// It first checks dynamic speaker names (set from voice names), then falls back to
// the default "Alex"/"Sam"/"Jordan" mapping for backward compatibility with old scripts.
func VoiceForSpeaker(speaker string, voices VoiceMap) Voice {
	// Check dynamic speaker names first (position-based)
	if voices.SpeakerNames[0] != "" && speaker == voices.SpeakerNames[0] {
		return voices.Host1
	}
	if voices.SpeakerNames[1] != "" && speaker == voices.SpeakerNames[1] {
		return voices.Host2
	}
	if voices.SpeakerNames[2] != "" && speaker == voices.SpeakerNames[2] {
		return voices.Host3
	}

	// Fallback: hardcoded names for backward compat (--from-script with old scripts)
	switch speaker {
	case "Alex":
		return voices.Host1
	case "Sam":
		return voices.Host2
	case "Jordan":
		return voices.Host3
	default:
		return voices.Host1
	}
}
