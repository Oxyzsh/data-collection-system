package infrastructure

import "testing"

func TestBuildSystemCollectorTemplate_DefaultPointsEnabled(t *testing.T) {
	def := BuildSystemCollectorTemplate()
	if len(def.Points) == 0 {
		t.Fatalf("expected built-in system collector points")
	}
	for _, pt := range def.Points {
		if pt.Enabled != 1 {
			t.Fatalf("point_id=%d expected enabled=1, got %d", pt.PointID, pt.Enabled)
		}
	}
}

func TestLocalIOTemplatesContainEditableAlarm(t *testing.T) {
	tests := []struct {
		name string
		def  BuiltInTemplateDef
	}{
		{name: "DI", def: BuildLocalDITemplate()},
		{name: "DO", def: BuildLocalDOTemplate()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.def.Alarms) != 1 {
				t.Fatalf("%s alarm count = %d, want 1", tt.name, len(tt.def.Alarms))
			}
			alarm := tt.def.Alarms[0]
			if alarm.PointID != 1 || alarm.Name == "" || alarm.Enabled {
				t.Fatalf("%s alarm = %+v, want disabled alarm for point 1", tt.name, alarm)
			}
		})
	}
}
