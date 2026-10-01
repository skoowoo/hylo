# Knowledge Graph — Entity Type Distribution

Queried from `.hylo/meta.db` → `knowledge_links.source_entity_type` on 2026-06-26.

| Entity Type        | Count |
|--------------------|------:|
| concept            | 743   |
| person             | 207   |
| product            | 137   |
| company            | 108   |
| project            | 82    |
| topic              | 23    |
| book               | 22    |
| brand              | 15    |
| business-model     | 9     |
| tool               | 6     |
| role               | 6     |
| framework          | 5     |
| technique          | 4     |
| strategy           | 4     |
| protocol           | 4     |
| product-platform   | 4     |
| startup            | 3     |
| market             | 3     |
| opensource-project | 2     |
| service            | 1     |
| event              | 1     |
| disease            | 1     |
| community          | 1     |

## Graph Node Color Assignments

Top 8 types own distinct hues; the rest share nearby colors.

| Entity Type     | Color              | Hex       |
|-----------------|--------------------|-----------|
| concept         | yellow-400         | `#facc15` |
| person          | blue-400           | `#60a5fa` |
| product         | emerald-400        | `#34d399` |
| company         | orange-400         | `#fb923c` |
| project         | violet-400         | `#a78bfa` |
| topic           | pink-400           | `#f472b6` |
| book            | sky-400            | `#38bdf8` |
| brand           | red-400            | `#f87171` |
| business-model  | indigo-500         | `#6366f1` |
| tool            | teal-500           | `#14b8a6` |
| framework       | cyan-500           | `#06b6d4` |
| technique       | purple-500         | `#a855f7` |
| strategy        | red-500            | `#ef4444` |
| protocol        | cyan-400           | `#22d3ee` |
| product-platform| emerald-400        | `#34d399` |
| startup         | amber-500          | `#f59e0b` |
| role            | lime-400           | `#84cc16` |
| market          | orange-300         | `#fdba74` |
| opensource-project | green-500       | `#22c55e` |
| service         | teal-400           | `#2dd4bf` |
| event           | rose-500           | `#f43f5e` |
| disease         | red-600            | `#dc2626` |
| community       | blue-400           | `#60a5fa` |

Color definitions live in `internal/server/view/assets/graph.js` → `_entityTypeColors`.
