# prometheus-metric-parser
Utility to parse prometheus metrics and compare them against other metrics

Sample usage:
Single
```
prometheus-metric-parser single --file run/metrics-1 --metrics rox_central_sensor_event_duration
rox_central_sensor_event_duration Action=CREATE_RESOURCE Type=AlertResults  (2316931.820/20591) 112.522
rox_central_sensor_event_duration Action=CREATE_RESOURCE Type=Deployment  (35067.526/1000) 35.068
rox_central_sensor_event_duration Action=CREATE_RESOURCE Type=Namespace  (4.639/1) 4.639
rox_central_sensor_event_duration Action=CREATE_RESOURCE Type=Pod  (2470375.252/30640) 80.626
rox_central_sensor_event_duration Action=CREATE_RESOURCE Type=ProcessIndicator  (75900.908/124067) 0.612
rox_central_sensor_event_duration Action=REMOVE_RESOURCE Type=AlertResults  (813122.062/1000) 813.122
rox_central_sensor_event_duration Action=REMOVE_RESOURCE Type=Deployment  (643661.228/1000) 643.661
rox_central_sensor_event_duration Action=REMOVE_RESOURCE Type=Pod  (2605066.704/30640) 85.022
rox_central_sensor_event_duration Action=UNSET_ACTION_RESOURCE Type=ClusterStatusUpdate  (1.825/2) 0.913
rox_central_sensor_event_duration Action=UNSET_ACTION_RESOURCE Type=ReprocessDeployment  (2355775.354/13561) 173.717
rox_central_sensor_event_duration Action=UPDATE_RESOURCE Type=AlertResults  (1323609.459/5234) 252.887
rox_central_sensor_event_duration Action=UPDATE_RESOURCE Type=Deployment  (1034455.449/5257) 196.777
rox_central_sensor_event_duration Action=UPDATE_RESOURCE Type=Namespace  (3.804/1) 3.804
rox_central_sensor_event_duration Action=UPDATE_RESOURCE Type=Pod  (506333.994/5000) 101.267
```

Compare
```
prometheus-metric-parser compare --new-file rocks-2d-ui-clicking/metrics-1 --old-file master-commit-latest/metrics-1 --metrics rox_central_sensor_event_duration
rox_central_sensor_event_duration Action=CREATE_RESOURCE Type=AlertResults  (old: (7337589.02/22827) 321.44, new (2316931.82/20591) 112.52): change: -64.9949%
rox_central_sensor_event_duration Action=CREATE_RESOURCE Type=Deployment  (old: (780959.81/2501) 312.26, new (35067.53/1000) 35.07): change: -88.7697%
rox_central_sensor_event_duration Action=UPDATE_RESOURCE Type=Namespace  (old: (3.04/1) 3.04, new (3.80/1) 3.80): change: 25.2979%
rox_central_sensor_event_duration Action=UPDATE_RESOURCE Type=Pod  (old: (1209997.35/12500) 96.80, new (506333.99/5000) 101.27): change: 4.6147%
rox_central_sensor_event_duration Action=REMOVE_RESOURCE Type=Pod  (old: (10338396.54/75652) 136.66, new (2605066.70/30640) 85.02): change: -37.7847%
rox_central_sensor_event_duration Action=UNSET_ACTION_RESOURCE Type=ClusterStatusUpdate  (old: (1.86/2) 0.93, new (1.83/2) 0.91): change: -1.9899%
rox_central_sensor_event_duration Action=CREATE_RESOURCE Type=ProcessIndicator  (old: (11889.16/326354) 0.04, new (75900.91/124067) 0.61): change: 1579.3007%
rox_central_sensor_event_duration Action=REMOVE_RESOURCE Type=AlertResults  (old: (3135411.99/3272) 958.26, new (813122.06/1000) 813.12): change: -15.1456%
rox_central_sensor_event_duration Action=REMOVE_RESOURCE Type=Deployment  (old: (4731132.52/3168) 1493.41, new (643661.23/1000) 643.66): change: -56.9000%
rox_central_sensor_event_duration Action=UNSET_ACTION_RESOURCE Type=ReprocessDeployment  (old: (4907789.49/36457) 134.62, new (2355775.35/13561) 173.72): change: 29.0438%
rox_central_sensor_event_duration Action=CREATE_RESOURCE Type=Namespace  (old: (3.78/1) 3.78, new (4.64/1) 4.64): change: 22.6006%
rox_central_sensor_event_duration Action=CREATE_RESOURCE Type=Pod  (old: (13212793.84/75477) 175.06, new (2470375.25/30640) 80.63): change: -53.9432%
rox_central_sensor_event_duration Action=UPDATE_RESOURCE Type=AlertResults  (old: (7739267.29/7147) 1082.87, new (1323609.46/5234) 252.89): change: -76.6466%
rox_central_sensor_event_duration Action=UPDATE_RESOURCE Type=Deployment  (old: (10362886.65/27410) 378.07, new (1034455.45/5257) 196.78): change: -47.9522%
```
