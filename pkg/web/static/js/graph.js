/**
 * R19DEV Studio - Relationship & Knowledge Network Graph
 * Force-directed Canvas 2D engine visualizing connections between Actresses, Genres, and Studios.
 * Native ES Module - Zero External Dependencies
 */

import { state, escapeHtml } from './state.js';
import { filterCollectionActress, toggleActressGenreFilter } from './actress.js';

const graphState = {
  isOpen: false,
  canvas: null,
  ctx: null,
  animId: null,
  width: 0,
  height: 0,
  dpr: window.devicePixelRatio || 1,

  // Graph Dataset
  allNodes: [],
  allLinks: [],
  visibleNodes: [],
  visibleLinks: [],

  // Physics Simulation
  alpha: 1.0,
  isPaused: false,

  // Camera / Transform
  panX: 0,
  panY: 0,
  zoom: 1.0,
  targetPanX: 0,
  targetPanY: 0,
  targetZoom: 1.0,

  // Interactions
  isDraggingCanvas: false,
  isDraggingNode: false,
  dragNode: null,
  dragStartX: 0,
  dragStartY: 0,
  lastMouseX: 0,
  lastMouseY: 0,
  hoveredNode: null,
  selectedNode: null,

  // Visibility Toggles
  showActresses: true,
  showGenres: true,
  showStudios: true,
  searchQuery: '',

  // Image cache for avatar circles
  imageCache: new Map()
};

/**
 * Initialize DOM events and controls for the Network Graph modal
 */
export function setupNetworkGraph() {
  const modal = document.getElementById('modal-network-graph');
  if (!modal) return;

  graphState.canvas = document.getElementById('network-graph-canvas');
  if (graphState.canvas) {
    graphState.ctx = graphState.canvas.getContext('2d');
  }

  // Backdrop click to close
  modal.addEventListener('click', (e) => {
    if (e.target === modal) {
      closeNetworkGraph();
    }
  });

  // Close Button
  document.getElementById('btn-close-graph')?.addEventListener('click', closeNetworkGraph);

  // Inspector Close
  document.getElementById('btn-inspector-close')?.addEventListener('click', () => {
    graphState.selectedNode = null;
    const inspector = document.getElementById('graph-inspector');
    if (inspector) inspector.classList.add('hidden');
    wakeSimulation();
  });

  // Search Input
  const searchInput = document.getElementById('graph-search-input');
  const searchClear = document.getElementById('graph-search-clear');
  searchInput?.addEventListener('input', (e) => {
    graphState.searchQuery = (e.target.value || '').trim().toLowerCase();
    searchClear?.classList.toggle('hidden', !graphState.searchQuery);
    applyFilterAndSearch();
  });

  searchClear?.addEventListener('click', () => {
    if (searchInput) searchInput.value = '';
    graphState.searchQuery = '';
    searchClear.classList.add('hidden');
    applyFilterAndSearch();
  });

  // Filter Toggles
  const toggleActress = document.getElementById('graph-toggle-actress');
  const toggleGenre = document.getElementById('graph-toggle-genre');
  const toggleStudio = document.getElementById('graph-toggle-studio');

  toggleActress?.addEventListener('change', (e) => {
    graphState.showActresses = e.target.checked;
    applyFilterAndSearch();
  });

  toggleGenre?.addEventListener('change', (e) => {
    graphState.showGenres = e.target.checked;
    applyFilterAndSearch();
  });

  toggleStudio?.addEventListener('change', (e) => {
    graphState.showStudios = e.target.checked;
    applyFilterAndSearch();
  });

  // Floating Controls
  document.getElementById('btn-graph-zoom-in')?.addEventListener('click', () => {
    zoomBy(1.25);
  });

  document.getElementById('btn-graph-zoom-out')?.addEventListener('click', () => {
    zoomBy(0.8);
  });

  document.getElementById('btn-graph-reset')?.addEventListener('click', () => {
    resetCamera();
  });

  document.getElementById('btn-graph-pause')?.addEventListener('click', () => {
    graphState.isPaused = !graphState.isPaused;
    const icon = document.getElementById('icon-graph-pause');
    if (icon) {
      icon.textContent = graphState.isPaused ? 'play_arrow' : 'pause';
    }
    if (!graphState.isPaused) wakeSimulation();
  });

  // Canvas Mouse & Touch Interactions
  if (graphState.canvas) {
    setupCanvasInteractions(graphState.canvas);
  }

  // Window Resize
  window.addEventListener('resize', () => {
    if (graphState.isOpen) {
      resizeCanvas();
    }
  });
}

/**
 * Open the Network Graph modal, optionally auto-focusing on an actress or genre
 */
export function openNetworkGraph(focusNodeName = null) {
  const modal = document.getElementById('modal-network-graph');
  if (!modal) return;

  modal.classList.remove('hidden');
  graphState.isOpen = true;

  // Build Graph Data
  buildGraphData();

  // Resize canvas & initialize positions
  resizeCanvas();

  // Pre-calculate settled layout upfront (runs silently in ~10ms before visual paint)
  precomputeLayout(260);

  // Auto-fit camera around the settled nodes
  fitCameraToNodes();

  // If focus specified, center on it
  if (focusNodeName) {
    const match = graphState.visibleNodes.find(n => n.label.toLowerCase() === focusNodeName.toLowerCase());
    if (match) {
      selectNode(match);
      focusOnNode(match);
    }
  }

  renderGraph();
  startLoop();
}

/**
 * Close the Network Graph modal
 */
export function closeNetworkGraph() {
  const modal = document.getElementById('modal-network-graph');
  if (modal) modal.classList.add('hidden');

  graphState.isOpen = false;
  if (graphState.animId) {
    cancelAnimationFrame(graphState.animId);
    graphState.animId = null;
  }
}

/**
 * Construct graph nodes and edges from state.actresses
 */
function buildGraphData() {
  const nodesMap = new Map();
  const links = [];

  const actresses = state.actresses || [];
  if (actresses.length === 0) {
    graphState.allNodes = [];
    graphState.allLinks = [];
    graphState.visibleNodes = [];
    graphState.visibleLinks = [];
    return;
  }

  // 1. Actress Nodes
  actresses.forEach(entry => {
    const a = entry.actress;
    const releases = entry.releases || [];
    const inLibCount = releases.filter(r => r.organized_folder || r.library_path || r.is_downloaded || state.organizedStatus[r.movie_id]).length;
    const nodeId = 'actress:' + a.name;

    const node = {
      id: nodeId,
      type: 'actress',
      label: a.name,
      jaName: a.ja_name || '',
      avatar: a.image_url || '',
      worksCount: releases.length,
      inLibCount: inLibCount,
      topGenres: (entry.top_genres || []).slice(0, 5),
      radius: Math.max(18, Math.min(32, 16 + Math.sqrt(releases.length) * 2.8)),
      x: 0,
      y: 0,
      vx: 0,
      vy: 0
    };

    nodesMap.set(nodeId, node);

    // Preload avatar
    if (a.image_url && !graphState.imageCache.has(a.image_url)) {
      const img = new Image();
      img.src = a.image_url;
      graphState.imageCache.set(a.image_url, img);
    }
  });

  // 2. Genre & Studio Tallies
  const genreTally = {};
  const studioTally = {};
  const actressGenreWeights = new Map();
  const actressStudioWeights = new Map();
  const movieCoStarMap = new Map(); // movie_id -> [actress_names]

  actresses.forEach(entry => {
    const actName = entry.actress.name;
    const releases = entry.releases || [];

    releases.forEach(rel => {
      // Co-star map
      if (rel.movie_id) {
        if (!movieCoStarMap.has(rel.movie_id)) movieCoStarMap.set(rel.movie_id, new Set());
        movieCoStarMap.get(rel.movie_id).add(actName);
      }

      // Genres
      (rel.genres || []).forEach(g => {
        const trimmed = (g || '').trim();
        if (!trimmed) return;
        genreTally[trimmed] = (genreTally[trimmed] || 0) + 1;

        const key = `${actName}:::${trimmed}`;
        actressGenreWeights.set(key, (actressGenreWeights.get(key) || 0) + 1);
      });

      // Studios
      const maker = (rel.maker || '').trim();
      if (maker) {
        studioTally[maker] = (studioTally[maker] || 0) + 1;
        const key = `${actName}:::${maker}`;
        actressStudioWeights.set(key, (actressStudioWeights.get(key) || 0) + 1);
      }
    });
  });

  // Filter Top Genres (keep top 24 or genres with >= 2 count)
  const sortedGenres = Object.entries(genreTally)
    .filter(([_, count]) => count >= 2)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 24);

  sortedGenres.forEach(([genre, count]) => {
    const nodeId = 'genre:' + genre;
    const node = {
      id: nodeId,
      type: 'genre',
      label: genre,
      worksCount: count,
      actresses: [],
      radius: Math.max(13, Math.min(26, 11 + Math.sqrt(count) * 2.2)),
      x: 0,
      y: 0,
      vx: 0,
      vy: 0
    };
    nodesMap.set(nodeId, node);
  });

  // Filter Top Studios (top 16 or studios with >= 2 count)
  const sortedStudios = Object.entries(studioTally)
    .filter(([_, count]) => count >= 2)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 16);

  sortedStudios.forEach(([studio, count]) => {
    const nodeId = 'studio:' + studio;
    const node = {
      id: nodeId,
      type: 'studio',
      label: studio,
      worksCount: count,
      actresses: [],
      radius: Math.max(12, Math.min(22, 10 + Math.sqrt(count) * 1.8)),
      x: 0,
      y: 0,
      vx: 0,
      vy: 0
    };
    nodesMap.set(nodeId, node);
  });

  // 3. Connect Actress <-> Genre Links
  actressGenreWeights.forEach((weight, key) => {
    const [actName, genre] = key.split(':::');
    const actNode = nodesMap.get('actress:' + actName);
    const genreNode = nodesMap.get('genre:' + genre);

    if (actNode && genreNode) {
      genreNode.actresses.push({ name: actName, count: weight });
      links.push({
        source: actNode,
        target: genreNode,
        weight: weight,
        type: 'actress-genre'
      });
    }
  });

  // 4. Connect Actress <-> Studio Links
  actressStudioWeights.forEach((weight, key) => {
    const [actName, studio] = key.split(':::');
    const actNode = nodesMap.get('actress:' + actName);
    const studioNode = nodesMap.get('studio:' + studio);

    if (actNode && studioNode) {
      studioNode.actresses.push({ name: actName, count: weight });
      links.push({
        source: actNode,
        target: studioNode,
        weight: weight,
        type: 'actress-studio'
      });
    }
  });

  // 5. Connect Actress <-> Actress (Co-stars)
  const addedCoStarPairs = new Set();
  movieCoStarMap.forEach(actsSet => {
    if (actsSet.size >= 2) {
      const arr = Array.from(actsSet);
      for (let i = 0; i < arr.length; i++) {
        for (let j = i + 1; j < arr.length; j++) {
          const pairKey = [arr[i], arr[j]].sort().join(':::');
          if (!addedCoStarPairs.has(pairKey)) {
            addedCoStarPairs.add(pairKey);
            const a1 = nodesMap.get('actress:' + arr[i]);
            const a2 = nodesMap.get('actress:' + arr[j]);
            if (a1 && a2) {
              links.push({
                source: a1,
                target: a2,
                weight: 2,
                type: 'co-star'
              });
            }
          }
        }
      }
    }
  });

  // Layout initialization in a ring
  const allNodes = Array.from(nodesMap.values());
  const total = allNodes.length;
  allNodes.forEach((node, idx) => {
    const angle = (idx / total) * Math.PI * 2;
    const dist = 140 + Math.random() * 260;
    node.x = Math.cos(angle) * dist;
    node.y = Math.sin(angle) * dist;
    node.vx = (Math.random() - 0.5) * 2;
    node.vy = (Math.random() - 0.5) * 2;
  });

  graphState.allNodes = allNodes;
  graphState.allLinks = links;

  applyFilterAndSearch();
}

/**
 * Filter visible nodes/links based on toggle checkboxes and search query
 */
function applyFilterAndSearch() {
  const q = graphState.searchQuery;

  graphState.visibleNodes = graphState.allNodes.filter(node => {
    if (node.type === 'actress' && !graphState.showActresses) return false;
    if (node.type === 'genre' && !graphState.showGenres) return false;
    if (node.type === 'studio' && !graphState.showStudios) return false;

    if (q) {
      const matchLabel = node.label.toLowerCase().includes(q);
      const matchJa = (node.jaName || '').toLowerCase().includes(q);
      return matchLabel || matchJa;
    }
    return true;
  });

  const visibleNodeIds = new Set(graphState.visibleNodes.map(n => n.id));

  graphState.visibleLinks = graphState.allLinks.filter(link => {
    return visibleNodeIds.has(link.source.id) && visibleNodeIds.has(link.target.id);
  });

  wakeSimulation();
}

/**
 * Resize canvas to match display size with high DPI support
 */
function resizeCanvas() {
  const container = document.getElementById('graph-canvas-container');
  if (!container || !graphState.canvas) return;

  const rect = container.getBoundingClientRect();
  graphState.width = rect.width || 800;
  graphState.height = rect.height || 600;

  graphState.canvas.width = graphState.width * graphState.dpr;
  graphState.canvas.height = graphState.height * graphState.dpr;

  if (graphState.ctx) {
    graphState.ctx.setTransform(graphState.dpr, 0, 0, graphState.dpr, 0, 0);
  }
}

/**
 * Precompute layout simulation upfront so nodes reach steady state
 * and do not bounce or fly around when viewed.
 */
function precomputeLayout(iterations = 260) {
  const savePaused = graphState.isPaused;
  graphState.isPaused = false;
  graphState.alpha = 1.0;

  for (let iter = 0; iter < iterations; iter++) {
    updatePhysics();
    if (graphState.alpha < 0.005) break;
  }

  // Freeze any residual velocities so nodes are stationary on open
  graphState.allNodes.forEach(node => {
    node.vx = 0;
    node.vy = 0;
  });

  graphState.isPaused = savePaused;
  graphState.alpha = 0.02; // Very gentle micro-settle for initial frames
}

/**
 * Auto-fit camera around visible nodes with comfortable padding
 */
function fitCameraToNodes() {
  const nodes = graphState.visibleNodes;
  if (!nodes || nodes.length === 0) {
    graphState.panX = graphState.width / 2;
    graphState.panY = graphState.height / 2;
    graphState.zoom = 1.0;
    return;
  }

  let minX = Infinity, maxX = -Infinity;
  let minY = Infinity, maxY = -Infinity;

  nodes.forEach(node => {
    const r = node.radius || 20;
    if (node.x - r < minX) minX = node.x - r;
    if (node.x + r > maxX) maxX = node.x + r;
    if (node.y - r < minY) minY = node.y - r;
    if (node.y + r > maxY) maxY = node.y + r;
  });

  const graphW = maxX - minX;
  const graphH = maxY - minY;
  const cx = (minX + maxX) / 2;
  const cy = (minY + maxY) / 2;

  const pad = 120;
  const availW = Math.max(200, graphState.width - pad);
  const availH = Math.max(200, graphState.height - pad);

  let autoZoom = Math.min(availW / Math.max(100, graphW), availH / Math.max(100, graphH));
  autoZoom = Math.max(0.4, Math.min(1.2, autoZoom));

  graphState.zoom = autoZoom;
  graphState.panX = (graphState.width / 2) - cx * autoZoom;
  graphState.panY = (graphState.height / 2) - cy * autoZoom;
  graphState.targetZoom = graphState.zoom;
  graphState.targetPanX = graphState.panX;
  graphState.targetPanY = graphState.panY;
}

/**
 * Reset camera to center the graph
 */
function resetCamera() {
  fitCameraToNodes();
  renderGraph();
}

/**
 * Smoothly zoom by a factor centered on canvas center
 */
function zoomBy(factor) {
  const newZoom = Math.max(0.2, Math.min(3.5, graphState.zoom * factor));
  const cx = graphState.width / 2;
  const cy = graphState.height / 2;

  graphState.panX = cx - ((cx - graphState.panX) * (newZoom / graphState.zoom));
  graphState.panY = cy - ((cy - graphState.panY) * (newZoom / graphState.zoom));
  graphState.zoom = newZoom;
  renderGraph();
}

/**
 * Focus and zoom on a specific node
 */
function focusOnNode(node) {
  graphState.zoom = 1.4;
  graphState.panX = graphState.width / 2 - node.x * graphState.zoom;
  graphState.panY = graphState.height / 2 - node.y * graphState.zoom;
  renderGraph();
}

/**
 * Wake physics simulation up to run cooling cycles
 */
function wakeSimulation(strength = 0.18) {
  graphState.alpha = Math.max(graphState.alpha, strength);
  if (!graphState.animId && graphState.isOpen) {
    startLoop();
  }
}

/**
 * Physics update step
 */
function updatePhysics() {
  if (graphState.isPaused) return;

  const nodes = graphState.visibleNodes;
  const links = graphState.visibleLinks;
  const n = nodes.length;
  if (n === 0) return;

  // 1. Repulsion force between all node pairs
  for (let i = 0; i < n; i++) {
    const ni = nodes[i];
    for (let j = i + 1; j < n; j++) {
      const nj = nodes[j];
      const dx = nj.x - ni.x;
      const dy = nj.y - ni.y;
      let distSq = dx * dx + dy * dy;
      if (distSq < 1) distSq = 1;
      const dist = Math.sqrt(distSq);

      const minDist = ni.radius + nj.radius + 35;
      if (dist < 420) {
        let force = (minDist * minDist * 0.9) / (distSq * dist);
        if (force > 5.0) force = 5.0; // clamp maximum repulsion force
        const fx = dx * force;
        const fy = dy * force;
        ni.vx -= fx;
        ni.vy -= fy;
        nj.vx += fx;
        nj.vy += fy;
      }
    }
  }

  // 2. Spring force along links
  for (let k = 0; k < links.length; k++) {
    const link = links[k];
    const s = link.source;
    const t = link.target;
    const dx = t.x - s.x;
    const dy = t.y - s.y;
    const dist = Math.sqrt(dx * dx + dy * dy) || 1;

    let targetDist = s.radius + t.radius + 65;
    if (link.type === 'co-star') targetDist = 95;
    if (link.type === 'actress-studio') targetDist = 80;

    const diff = dist - targetDist;
    const force = diff * 0.025;
    const fx = (dx / dist) * force;
    const fy = (dy / dist) * force;

    s.vx += fx;
    s.vy += fy;
    t.vx -= fx;
    t.vy -= fy;
  }

  // 3. Center gravity & integrate positions
  for (let i = 0; i < n; i++) {
    const node = nodes[i];
    node.vx += (-node.x) * 0.0025;
    node.vy += (-node.y) * 0.0025;

    // Velocity damping
    node.vx *= 0.85;
    node.vy *= 0.85;

    // Clamp velocity to avoid jittering
    const maxV = 10;
    if (node.vx > maxV) node.vx = maxV;
    else if (node.vx < -maxV) node.vx = -maxV;
    if (node.vy > maxV) node.vy = maxV;
    else if (node.vy < -maxV) node.vy = -maxV;

    if (node !== graphState.dragNode) {
      node.x += node.vx * graphState.alpha;
      node.y += node.vy * graphState.alpha;
    }
  }

  // Cooling decay
  graphState.alpha *= 0.991;
}

/**
 * Main animation & rendering loop
 */
function startLoop() {
  function loop() {
    if (!graphState.isOpen) return;

    updatePhysics();
    renderGraph();

    if (graphState.alpha > 0.004 || graphState.isDraggingCanvas || graphState.isDraggingNode) {
      graphState.animId = requestAnimationFrame(loop);
    } else {
      // Idle state reached -> stop animating to keep CPU 0%
      graphState.animId = null;
      renderGraph(); // Final crisp draw
    }
  }

  if (graphState.animId) cancelAnimationFrame(graphState.animId);
  graphState.animId = requestAnimationFrame(loop);
}

/**
 * Render the entire network graph on HTML5 Canvas
 */
function renderGraph() {
  const ctx = graphState.ctx;
  if (!ctx) return;

  const w = graphState.width;
  const h = graphState.height;

  ctx.clearRect(0, 0, w, h);

  // Background
  ctx.save();
  ctx.fillStyle = '#0b0d14';
  ctx.fillRect(0, 0, w, h);

  // Apply Camera Transform
  ctx.translate(graphState.panX, graphState.panY);
  ctx.scale(graphState.zoom, graphState.zoom);

  const hovered = graphState.hoveredNode;
  const selected = graphState.selectedNode;
  const activeFocus = hovered || selected;

  // Determine connected nodes if a node is focused
  const connectedNodeIds = new Set();
  if (activeFocus) {
    connectedNodeIds.add(activeFocus.id);
    graphState.visibleLinks.forEach(link => {
      if (link.source.id === activeFocus.id) connectedNodeIds.add(link.target.id);
      if (link.target.id === activeFocus.id) connectedNodeIds.add(link.source.id);
    });
  }

  // 1. Draw Links
  graphState.visibleLinks.forEach(link => {
    const s = link.source;
    const t = link.target;
    const isConnected = activeFocus && (s.id === activeFocus.id || t.id === activeFocus.id);

    ctx.beginPath();
    ctx.moveTo(s.x, s.y);
    ctx.lineTo(t.x, t.y);

    if (activeFocus) {
      if (isConnected) {
        ctx.strokeStyle = link.type === 'co-star' ? '#818cf8' : link.type === 'actress-studio' ? '#38bdf8' : '#34d399';
        ctx.lineWidth = Math.min(4, Math.max(2, link.weight));
        ctx.globalAlpha = 0.95;
      } else {
        ctx.strokeStyle = 'rgba(255, 255, 255, 0.03)';
        ctx.lineWidth = 1;
        ctx.globalAlpha = 0.15;
      }
    } else {
      if (link.type === 'co-star') {
        ctx.strokeStyle = '#6366f1';
        ctx.globalAlpha = 0.35;
        ctx.lineWidth = 1.5;
      } else if (link.type === 'actress-studio') {
        ctx.strokeStyle = '#0284c7';
        ctx.globalAlpha = 0.25;
        ctx.lineWidth = 1.2;
      } else {
        ctx.strokeStyle = 'rgba(255, 255, 255, 0.14)';
        ctx.globalAlpha = 0.45;
        ctx.lineWidth = Math.min(3, Math.max(1, link.weight * 0.7));
      }
    }

    ctx.stroke();
    ctx.globalAlpha = 1.0;
  });

  // 2. Draw Nodes
  graphState.visibleNodes.forEach(node => {
    const isNodeActive = activeFocus ? connectedNodeIds.has(node.id) : true;
    const isPrimaryFocus = activeFocus && activeFocus.id === node.id;

    ctx.save();
    if (activeFocus && !isNodeActive) {
      ctx.globalAlpha = 0.2;
    }

    if (node.type === 'actress') {
      drawActressNode(ctx, node, isPrimaryFocus);
    } else if (node.type === 'genre') {
      drawGenreNode(ctx, node, isPrimaryFocus);
    } else if (node.type === 'studio') {
      drawStudioNode(ctx, node, isPrimaryFocus);
    }

    ctx.restore();
  });

  ctx.restore();
}

/**
 * Draw Performer Node with Avatar Image Clipping
 */
function drawActressNode(ctx, node, isFocus) {
  const r = node.radius;

  // Glow if focused
  if (isFocus) {
    ctx.shadowColor = '#34d399';
    ctx.shadowBlur = 18;
  }

  // Draw Avatar Image
  ctx.save();
  ctx.beginPath();
  ctx.arc(node.x, node.y, r, 0, Math.PI * 2);
  ctx.clip();

  const img = graphState.imageCache.get(node.avatar);
  if (img && img.complete && img.naturalWidth > 0) {
    ctx.drawImage(img, node.x - r, node.y - r, r * 2, r * 2);
  } else {
    // Fallback circle
    ctx.fillStyle = '#064e3b';
    ctx.fill();
    ctx.fillStyle = '#34d399';
    ctx.font = `bold ${Math.max(10, r * 0.85)}px sans-serif`;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(node.label.charAt(0).toUpperCase(), node.x, node.y);
  }
  ctx.restore();

  // Border Ring
  ctx.beginPath();
  ctx.arc(node.x, node.y, r, 0, Math.PI * 2);
  ctx.lineWidth = isFocus ? 3.5 : 2;
  ctx.strokeStyle = isFocus ? '#34d399' : '#059669';
  ctx.stroke();

  // Completed badge dot if 100% in library
  if (node.worksCount > 0 && node.inLibCount >= node.worksCount) {
    ctx.beginPath();
    ctx.arc(node.x + r * 0.7, node.y - r * 0.7, 5.5, 0, Math.PI * 2);
    ctx.fillStyle = '#10b981';
    ctx.fill();
    ctx.lineWidth = 1.5;
    ctx.strokeStyle = '#064e3b';
    ctx.stroke();
  }

  // Label Below
  ctx.shadowBlur = 0;
  ctx.font = `${isFocus ? 'bold 12px' : '11px'} Inter, sans-serif`;
  ctx.fillStyle = isFocus ? '#34d399' : '#e2e8f0';
  ctx.textAlign = 'center';
  ctx.textBaseline = 'top';
  ctx.fillText(node.label, node.x, node.y + r + 5);
}

/**
 * Draw Genre Node (Violet Hexagon / Pill)
 */
function drawGenreNode(ctx, node, isFocus) {
  const r = node.radius;

  if (isFocus) {
    ctx.shadowColor = '#a78bfa';
    ctx.shadowBlur = 18;
  }

  // Hexagon Shape
  ctx.beginPath();
  const sides = 6;
  for (let i = 0; i < sides; i++) {
    const a = (i * Math.PI * 2) / sides - Math.PI / 6;
    const px = node.x + r * Math.cos(a);
    const py = node.y + r * Math.sin(a);
    if (i === 0) ctx.moveTo(px, py);
    else ctx.lineTo(px, py);
  }
  ctx.closePath();

  ctx.fillStyle = isFocus ? '#4c1d95' : 'rgba(76, 29, 149, 0.85)';
  ctx.fill();
  ctx.lineWidth = isFocus ? 2.5 : 1.5;
  ctx.strokeStyle = isFocus ? '#c4b5fd' : '#8b5cf6';
  ctx.stroke();

  // Label below
  ctx.shadowBlur = 0;
  ctx.font = '10px Inter, sans-serif';
  ctx.fillStyle = isFocus ? '#c4b5fd' : '#a78bfa';
  ctx.textAlign = 'center';
  ctx.textBaseline = 'top';

  const shortLabel = node.label.length > 11 ? node.label.substring(0, 10) + '…' : node.label;
  ctx.fillText(shortLabel, node.x, node.y + r + 4);
}

/**
 * Draw Studio / Maker Node (Cyan Rounded Square)
 */
function drawStudioNode(ctx, node, isFocus) {
  const r = node.radius;

  if (isFocus) {
    ctx.shadowColor = '#38bdf8';
    ctx.shadowBlur = 18;
  }

  const s = r * 1.5;
  const rx = node.x - s / 2;
  const ry = node.y - s / 2;

  ctx.beginPath();
  if (ctx.roundRect) {
    ctx.roundRect(rx, ry, s, s, 6);
  } else {
    ctx.rect(rx, ry, s, s);
  }
  ctx.fillStyle = isFocus ? '#0369a1' : 'rgba(3, 105, 161, 0.85)';
  ctx.fill();
  ctx.lineWidth = isFocus ? 2.5 : 1.5;
  ctx.strokeStyle = isFocus ? '#7dd3fc' : '#0284c7';
  ctx.stroke();

  ctx.shadowBlur = 0;
  ctx.font = '10px Inter, sans-serif';
  ctx.fillStyle = isFocus ? '#7dd3fc' : '#38bdf8';
  ctx.textAlign = 'center';
  ctx.textBaseline = 'top';

  const shortLabel = node.label.length > 11 ? node.label.substring(0, 10) + '…' : node.label;
  ctx.fillText(shortLabel, node.x, node.y + s / 2 + 4);
}

/**
 * Handle mouse/touch canvas interactions: pan, zoom, hover, click, drag
 */
function setupCanvasInteractions(canvas) {
  // Screen to World Coordinates
  function screenToWorld(sx, sy) {
    const wx = (sx - graphState.panX) / graphState.zoom;
    const wy = (sy - graphState.panY) / graphState.zoom;
    return { x: wx, y: wy };
  }

  // Find Node at coordinate
  function findNodeAt(wx, wy) {
    for (let i = graphState.visibleNodes.length - 1; i >= 0; i--) {
      const node = graphState.visibleNodes[i];
      const dx = node.x - wx;
      const dy = node.y - wy;
      if (dx * dx + dy * dy <= (node.radius + 6) * (node.radius + 6)) {
        return node;
      }
    }
    return null;
  }

  // Mouse Move
  canvas.addEventListener('mousemove', (e) => {
    const rect = canvas.getBoundingClientRect();
    const mx = e.clientX - rect.left;
    const my = e.clientY - rect.top;

    if (graphState.isDraggingNode && graphState.dragNode) {
      const world = screenToWorld(mx, my);
      graphState.dragNode.x = world.x;
      graphState.dragNode.y = world.y;
      graphState.dragNode.vx = 0;
      graphState.dragNode.vy = 0;
      wakeSimulation(0.25);
      return;
    }

    if (graphState.isDraggingCanvas) {
      const dx = mx - graphState.lastMouseX;
      const dy = my - graphState.lastMouseY;
      graphState.panX += dx;
      graphState.panY += dy;
      graphState.lastMouseX = mx;
      graphState.lastMouseY = my;
      renderGraph();
      return;
    }

    // Hover detection
    const world = screenToWorld(mx, my);
    const node = findNodeAt(world.x, world.y);

    if (node !== graphState.hoveredNode) {
      graphState.hoveredNode = node;
      canvas.style.cursor = node ? 'pointer' : 'grab';
      renderGraph();
    }
  });

  // Mouse Down
  canvas.addEventListener('mousedown', (e) => {
    if (e.button !== 0) return;
    const rect = canvas.getBoundingClientRect();
    const mx = e.clientX - rect.left;
    const my = e.clientY - rect.top;

    const world = screenToWorld(mx, my);
    const node = findNodeAt(world.x, world.y);

    graphState.dragStartX = mx;
    graphState.dragStartY = my;
    graphState.lastMouseX = mx;
    graphState.lastMouseY = my;

    if (node) {
      graphState.isDraggingNode = true;
      graphState.dragNode = node;
      canvas.style.cursor = 'grabbing';
      wakeSimulation(0.25);
    } else {
      graphState.isDraggingCanvas = true;
      canvas.style.cursor = 'grabbing';
    }
  });

  // Mouse Up
  window.addEventListener('mouseup', (e) => {
    if (!graphState.isOpen) return;

    const rect = canvas.getBoundingClientRect();
    const mx = e.clientX - rect.left;
    const my = e.clientY - rect.top;

    const movedDist = Math.hypot(mx - graphState.dragStartX, my - graphState.dragStartY);

    if (graphState.isDraggingNode) {
      graphState.isDraggingNode = false;
      if (movedDist < 5 && graphState.dragNode) {
        selectNode(graphState.dragNode);
      }
      graphState.dragNode = null;
      canvas.style.cursor = 'pointer';
    } else if (graphState.isDraggingCanvas) {
      graphState.isDraggingCanvas = false;
      canvas.style.cursor = 'grab';
      if (movedDist < 5) {
        const world = screenToWorld(mx, my);
        const node = findNodeAt(world.x, world.y);
        if (!node) {
          selectNode(null);
        }
      }
    }
  });

  // Mouse Wheel: Zoom centered at mouse position
  canvas.addEventListener('wheel', (e) => {
    e.preventDefault();
    const rect = canvas.getBoundingClientRect();
    const mx = e.clientX - rect.left;
    const my = e.clientY - rect.top;

    const zoomFactor = e.deltaY < 0 ? 1.12 : 0.89;
    const newZoom = Math.max(0.18, Math.min(3.8, graphState.zoom * zoomFactor));

    graphState.panX = mx - ((mx - graphState.panX) * (newZoom / graphState.zoom));
    graphState.panY = my - ((my - graphState.panY) * (newZoom / graphState.zoom));
    graphState.zoom = newZoom;

    renderGraph();
  }, { passive: false });
}

/**
 * Select a node and show details in the Inspector Drawer
 */
function selectNode(node) {
  graphState.selectedNode = node;
  const inspector = document.getElementById('graph-inspector');
  const body = document.getElementById('graph-inspector-body');

  if (!inspector || !body) return;

  if (!node) {
    inspector.classList.add('hidden');
    wakeSimulation();
    return;
  }

  inspector.classList.remove('hidden');

  if (node.type === 'actress') {
    const avatar = node.avatar || '/placeholder.png';
    const pct = node.worksCount > 0 ? Math.round((node.inLibCount / node.worksCount) * 100) : 0;

    body.innerHTML = `
      <div class="inspector-header">
        <img class="inspector-avatar" src="${avatar}" alt="${escapeHtml(node.label)}" onerror="this.src='/placeholder.png'" />
        <div class="inspector-title-wrap">
          <span class="inspector-tag tag-actress">ACTRESS</span>
          <h3 class="inspector-name">${escapeHtml(node.label)}</h3>
          ${node.jaName ? `<div class="inspector-sub">${escapeHtml(node.jaName)}</div>` : ''}
        </div>
      </div>

      <div class="inspector-stat-grid">
        <div class="inspector-stat-box">
          <span class="stat-num">${node.worksCount}</span>
          <span class="stat-lbl">Works</span>
        </div>
        <div class="inspector-stat-box">
          <span class="stat-num" style="color: #34d399;">${node.inLibCount}</span>
          <span class="stat-lbl">In Library (${pct}%)</span>
        </div>
      </div>

      ${node.topGenres && node.topGenres.length > 0 ? `
        <div class="inspector-section">
          <div class="inspector-sec-title">Top Genres</div>
          <div class="inspector-chips">
            ${node.topGenres.map(g => `<span class="inspector-chip">${escapeHtml(g.genre)} (${g.count})</span>`).join('')}
          </div>
        </div>
      ` : ''}

      <div class="inspector-actions">
        <button class="btn btn-primary btn-block" onclick="window.app.closeNetworkGraph(); window.app.filterCollectionActress('${escapeHtml(node.label)}')">
          <span class="material-symbols-outlined icon">movie</span> View Filmography
        </button>
      </div>
    `;
  } else if (node.type === 'genre') {
    const acts = node.actresses || [];
    body.innerHTML = `
      <div class="inspector-header">
        <div class="inspector-icon-box box-genre">
          <span class="material-symbols-outlined icon">label</span>
        </div>
        <div class="inspector-title-wrap">
          <span class="inspector-tag tag-genre">GENRE TAG</span>
          <h3 class="inspector-name">${escapeHtml(node.label)}</h3>
          <div class="inspector-sub">${node.worksCount} Works in Library</div>
        </div>
      </div>

      <div class="inspector-section">
        <div class="inspector-sec-title">Actresses with this Genre (${acts.length})</div>
        <div class="inspector-list">
          ${acts.slice(0, 10).map(a => `
            <div class="inspector-list-item" onclick="window.app.closeNetworkGraph(); window.app.filterCollectionActress('${escapeHtml(a.name)}')">
              <span class="name">${escapeHtml(a.name)}</span>
              <span class="count">${a.count} works</span>
            </div>
          `).join('')}
        </div>
      </div>

      <div class="inspector-actions">
        <button class="btn btn-secondary btn-block" onclick="window.app.closeNetworkGraph(); window.app.toggleActressGenreFilter('${escapeHtml(node.label)}')">
          <span class="material-symbols-outlined icon">filter_alt</span> Filter Filmography by ${escapeHtml(node.label)}
        </button>
      </div>
    `;
  } else if (node.type === 'studio') {
    const acts = node.actresses || [];
    body.innerHTML = `
      <div class="inspector-header">
        <div class="inspector-icon-box box-studio">
          <span class="material-symbols-outlined icon">apartment</span>
        </div>
        <div class="inspector-title-wrap">
          <span class="inspector-tag tag-studio">STUDIO / MAKER</span>
          <h3 class="inspector-name">${escapeHtml(node.label)}</h3>
          <div class="inspector-sub">${node.worksCount} Works</div>
        </div>
      </div>

      <div class="inspector-section">
        <div class="inspector-sec-title">Actresses who worked here (${acts.length})</div>
        <div class="inspector-list">
          ${acts.slice(0, 10).map(a => `
            <div class="inspector-list-item" onclick="window.app.closeNetworkGraph(); window.app.filterCollectionActress('${escapeHtml(a.name)}')">
              <span class="name">${escapeHtml(a.name)}</span>
              <span class="count">${a.count} works</span>
            </div>
          `).join('')}
        </div>
      </div>
    `;
  }

  wakeSimulation();
}
