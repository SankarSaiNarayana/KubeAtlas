import { GraphEdge, GraphNode } from "../api/client";

const KIND_COLORS: Record<string, string> = {
  Ingress: "#a371f7",
  Service: "#58a6ff",
  Deployment: "#3fb950",
  ConfigMap: "#d29922",
  Secret: "#f85149",
  Pod: "#79c0ff",
};

type LayoutNode = GraphNode & { x: number; y: number };

function layoutNodes(nodes: GraphNode[], edges: GraphEdge[]): LayoutNode[] {
  const byId = new Map(nodes.map((n) => [n.id, n]));
  const depth = new Map<string, number>();

  for (const n of nodes) depth.set(n.id, 0);

  for (let i = 0; i < nodes.length; i++) {
    for (const e of edges) {
      const d = depth.get(e.source_id) ?? 0;
      const target = byId.get(e.target_id);
      if (target) {
        depth.set(e.target_id, Math.max(depth.get(e.target_id) ?? 0, d + 1));
      }
    }
  }

  const layers = new Map<number, GraphNode[]>();
  for (const n of nodes) {
    const d = depth.get(n.id) ?? 0;
    if (!layers.has(d)) layers.set(d, []);
    layers.get(d)!.push(n);
  }

  const w = 720;
  const h = 280;
  const maxLayer = Math.max(...layers.keys(), 0);

  const result: LayoutNode[] = [];
  for (const [d, layerNodes] of layers) {
    layerNodes.forEach((n, i) => {
      const x = maxLayer === 0 ? w / 2 : 80 + (d / maxLayer) * (w - 160);
      const y = 40 + ((i + 1) / (layerNodes.length + 1)) * (h - 80);
      result.push({ ...n, x, y });
    });
  }
  return result;
}

export default function GraphCanvas({
  nodes,
  edges,
}: {
  nodes: GraphNode[];
  edges: GraphEdge[];
}) {
  if (nodes.length === 0) {
    return (
      <div className="graph-empty">
        <p>No resources in graph</p>
      </div>
    );
  }

  const laid = layoutNodes(nodes, edges);
  const pos = new Map(laid.map((n) => [n.id, n]));

  return (
    <svg className="graph-canvas" viewBox="0 0 720 280" role="img" aria-label="Dependency graph">
      <defs>
        <marker
          id="arrow"
          markerWidth="8"
          markerHeight="8"
          refX="6"
          refY="3"
          orient="auto"
        >
          <path d="M0,0 L6,3 L0,6 Z" fill="#484f58" />
        </marker>
      </defs>
      {edges.map((e) => {
        const s = pos.get(e.source_id);
        const t = pos.get(e.target_id);
        if (!s || !t) return null;
        return (
          <g key={e.id}>
            <line
              x1={s.x}
              y1={s.y}
              x2={t.x}
              y2={t.y}
              stroke="#484f58"
              strokeWidth="1.5"
              markerEnd="url(#arrow)"
            />
            <text
              x={(s.x + t.x) / 2}
              y={(s.y + t.y) / 2 - 6}
              fill="#8b949e"
              fontSize="9"
              textAnchor="middle"
            >
              {e.edge_type}
            </text>
          </g>
        );
      })}
      {laid.map((n) => (
        <g key={n.id} transform={`translate(${n.x}, ${n.y})`}>
          <rect
            x="-52"
            y="-22"
            width="104"
            height="44"
            rx="8"
            fill="#161b22"
            stroke={KIND_COLORS[n.kind] ?? "#30363d"}
            strokeWidth="2"
          />
          <text fill={KIND_COLORS[n.kind] ?? "#8b949e"} fontSize="10" fontWeight="600" textAnchor="middle" y="-4">
            {n.kind}
          </text>
          <text fill="#e6edf3" fontSize="11" fontWeight="500" textAnchor="middle" y="12">
            {n.name.length > 14 ? `${n.name.slice(0, 12)}…` : n.name}
          </text>
        </g>
      ))}
    </svg>
  );
}
