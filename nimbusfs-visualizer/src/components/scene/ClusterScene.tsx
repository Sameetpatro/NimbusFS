import { useRef, useMemo } from 'react';
import { useFrame } from '@react-three/fiber';
import { Float, Text, MeshDistortMaterial, Sphere } from '@react-three/drei';
import * as THREE from 'three';
import { STORAGE_NODES } from '../../lib/constants';
import { nodePosition } from '../../lib/chunking';
import { useStore } from '../../store/useStore';

const MASTER_POS: [number, number, number] = [0, 1.5, 0];
const CLIENT_POS: [number, number, number] = [0, 5.5, 0];

export function MasterNode() {
  const phase = useStore((s) => s.phase);
  const active = phase !== 'idle';

  const matRef = useRef<THREE.MeshStandardMaterial>(null);

  useFrame(({ clock }) => {
    if (matRef.current && active) {
      matRef.current.emissiveIntensity = 0.4 + Math.sin(clock.elapsedTime * 4) * 0.3;
    }
  });

  return (
    <group position={MASTER_POS}>
      <Float speed={2} rotationIntensity={0.3} floatIntensity={0.5}>
        <mesh castShadow>
          <octahedronGeometry args={[0.9, 0]} />
          <meshStandardMaterial
            ref={matRef}
            color="#6c5ce7"
            emissive="#a29bfe"
            emissiveIntensity={active ? 0.6 : 0.2}
            metalness={0.8}
            roughness={0.2}
          />
        </mesh>
      </Float>
      <Text position={[0, -1.4, 0]} fontSize={0.28} color="#dfe6e9" anchorX="center">
        MASTER
      </Text>
      <Text position={[0, -1.75, 0]} fontSize={0.14} color="#74b9ff" anchorX="center">
        REST :8080 · gRPC :9090
      </Text>
      {phase === 'metadata' && (
        <BoltDBFlash position={[0, 0, 0]} />
      )}
    </group>
  );
}

function BoltDBFlash({ position }: { position: [number, number, number] }) {
  const ref = useRef<THREE.Mesh>(null);
  useFrame(({ clock }) => {
    if (ref.current) {
      const s = 1 + Math.sin(clock.elapsedTime * 12) * 0.15;
      ref.current.scale.setScalar(s);
    }
  });
  return (
    <mesh ref={ref} position={position}>
      <ringGeometry args={[1.2, 1.5, 32]} />
      <meshBasicMaterial color="#00cec9" transparent opacity={0.5} side={THREE.DoubleSide} />
    </mesh>
  );
}

export function StorageNode3D({
  nodeId,
  label,
  angle,
  status,
  usedRatio,
}: {
  nodeId: string;
  label: string;
  angle: number;
  status: string;
  usedRatio: number;
}) {
  const pos = nodePosition(angle);
  const color = status === 'alive' ? '#00b894' : status === 'suspect' ? '#fdcb6e' : '#d63031';
  const chunks = useStore((s) => s.chunks);
  const activeChunk = useStore((s) => s.activeChunkIndex);
  const phase = useStore((s) => s.phase);

  const hasChunk = chunks.some(
    (c, i) => c.nodeIds.includes(nodeId as never) && (phase === 'complete' || i <= activeChunk),
  );

  const ringRef = useRef<THREE.Mesh>(null);
  const pulseRef = useRef<THREE.Mesh>(null);
  useFrame(({ clock }) => {
    if (ringRef.current && hasChunk) {
      ringRef.current.rotation.y = clock.elapsedTime * 0.8;
    }
    if (pulseRef.current && status === 'alive') {
      const s = 1 + Math.sin(clock.elapsedTime * 2 + angle) * 0.08;
      pulseRef.current.scale.setScalar(s);
      (pulseRef.current.material as THREE.MeshBasicMaterial).opacity =
        0.15 + Math.sin(clock.elapsedTime * 2 + angle) * 0.1;
    }
  });

  return (
    <group position={pos}>
      {status === 'alive' && (
        <mesh ref={pulseRef} rotation={[-Math.PI / 2, 0, 0]} position={[0, -0.1, 0]}>
          <ringGeometry args={[0.9, 1.1, 32]} />
          <meshBasicMaterial color={color} transparent opacity={0.15} side={THREE.DoubleSide} />
        </mesh>
      )}
      <mesh castShadow receiveShadow>
        <cylinderGeometry args={[0.55, 0.7, 0.5, 6]} />
        <meshStandardMaterial
          color="#2d3436"
          emissive={color}
          emissiveIntensity={hasChunk ? 0.5 : 0.15}
          metalness={0.6}
          roughness={0.4}
        />
      </mesh>
      {/* disk usage ring */}
      <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, 0.26, 0]}>
        <ringGeometry args={[0.5, 0.65, 32, 1, 0, Math.PI * 2 * usedRatio]} />
        <meshBasicMaterial color="#0984e3" transparent opacity={0.7} />
      </mesh>
      {hasChunk && (
        <mesh ref={ringRef} position={[0, 0.8, 0]}>
          <torusGeometry args={[0.35, 0.04, 8, 24]} />
          <meshBasicMaterial color="#00f5ff" />
        </mesh>
      )}
      <Text position={[0, -0.9, 0]} fontSize={0.22} color="#b2bec3" anchorX="center">
        {label}
      </Text>
      <Text position={[0, -1.2, 0]} fontSize={0.12} color={color} anchorX="center">
        {status.toUpperCase()}
      </Text>
    </group>
  );
}

export function StorageCluster() {
  const nodes = useStore((s) => s.nodes);

  return (
    <>
      {STORAGE_NODES.map((sn) => {
        const live = nodes.find((n) => n.id === sn.id);
        const usedRatio = live
          ? Math.min(1, live.usedSpace / live.totalSpace)
          : 0.3;
        return (
          <StorageNode3D
            key={sn.id}
            nodeId={sn.id}
            label={sn.label}
            angle={sn.angle}
            status={live?.status ?? 'alive'}
            usedRatio={usedRatio}
          />
        );
      })}
    </>
  );
}

export function ClientUploader() {
  const fileName = useStore((s) => s.fileName);
  const phase = useStore((s) => s.phase);
  const active = phase === 'uploading' || phase === 'chunking';

  return (
    <group position={CLIENT_POS}>
      <Float speed={1.5} floatIntensity={0.3}>
        <mesh>
          <boxGeometry args={[1.2, 0.8, 0.15]} />
          <meshStandardMaterial
            color="#dfe6e9"
            emissive="#74b9ff"
            emissiveIntensity={active ? 0.8 : 0.1}
            metalness={0.3}
            roughness={0.5}
          />
        </mesh>
      </Float>
      <Text position={[0, 0.7, 0]} fontSize={0.2} color="#fff" anchorX="center">
        CLIENT
      </Text>
      {fileName && (
        <Text position={[0, -0.7, 0]} fontSize={0.14} color="#81ecec" anchorX="center" maxWidth={3}>
          {fileName}
        </Text>
      )}
    </group>
  );
}

export function ChunkOrbit() {
  const chunks = useStore((s) => s.chunks);
  const activeChunk = useStore((s) => s.activeChunkIndex);
  const phase = useStore((s) => s.phase);

  if (phase === 'idle' || chunks.length === 0) return null;

  return (
    <group position={MASTER_POS}>
      {chunks.map((chunk, i) => {
        const visible =
          phase === 'complete' ||
          phase === 'metadata' ||
          (activeChunk >= 0 && i <= activeChunk);
        if (!visible) return null;
        const angle = (i / chunks.length) * Math.PI * 2;
        const r = 1.8;
        return (
          <ChunkCube
            key={chunk.index}
            position={[Math.cos(angle) * r, 0.3 + i * 0.1, Math.sin(angle) * r]}
            index={chunk.index}
            active={i === activeChunk}
            hash={chunk.chunkId}
          />
        );
      })}
    </group>
  );
}

function ChunkCube({
  position,
  index,
  active,
  hash,
}: {
  position: [number, number, number];
  index: number;
  active: boolean;
  hash: string;
}) {
  const ref = useRef<THREE.Mesh>(null);
  useFrame(({ clock }) => {
    if (ref.current) {
      ref.current.rotation.y = clock.elapsedTime * (active ? 3 : 0.5);
      ref.current.rotation.x = Math.sin(clock.elapsedTime) * 0.2;
    }
  });

  const colors = ['#00f5ff', '#7b61ff', '#ff6bcb', '#ffd166', '#06d6a0'];
  const color = colors[index % colors.length];

  return (
    <group position={position}>
      <mesh ref={ref} scale={active ? 1.2 : 0.7}>
        <boxGeometry args={[0.35, 0.35, 0.35]} />
        <MeshDistortMaterial
          color={color}
          emissive={color}
          emissiveIntensity={active ? 1.2 : 0.4}
          distort={active ? 0.4 : 0.1}
          speed={active ? 4 : 1}
        />
      </mesh>
      {active && (
        <Text position={[0, 0.5, 0]} fontSize={0.08} color="#fff" anchorX="center" maxWidth={1.5}>
          {hash.slice(0, 10)}…
        </Text>
      )}
    </group>
  );
}

export function DataPackets() {
  const packets = useStore((s) => s.packets);
  const nodeMap = useMemo(() => {
    const m = new Map<string, [number, number, number]>();
    STORAGE_NODES.forEach((n) => m.set(n.id, nodePosition(n.angle)));
    return m;
  }, []);

  return (
    <>
      {packets.map((pkt) => {
        const target = nodeMap.get(pkt.targetNodeId);
        if (!target) return null;
        const t = pkt.progress;
        const pos: [number, number, number] = [
          MASTER_POS[0] + (target[0] - MASTER_POS[0]) * t,
          MASTER_POS[1] + (target[1] - MASTER_POS[1]) * t + Math.sin(t * Math.PI) * 1.5,
          MASTER_POS[2] + (target[2] - MASTER_POS[2]) * t,
        ];
        return (
          <group key={pkt.id} position={pos}>
            <Sphere args={[0.12, 16, 16]}>
              <meshBasicMaterial color={pkt.color} />
            </Sphere>
            <pointLight color={pkt.color} intensity={2} distance={2} />
          </group>
        );
      })}
    </>
  );
}

export function ConnectionLines() {
  const lines = useMemo(() => {
    const pts: THREE.Vector3[][] = [];
    STORAGE_NODES.forEach((n) => {
      const end = new THREE.Vector3(...nodePosition(n.angle));
      const start = new THREE.Vector3(...MASTER_POS);
      const mid = start.clone().lerp(end, 0.5);
      mid.y += 0.5;
      const curve = new THREE.QuadraticBezierCurve3(start, mid, end);
      pts.push(curve.getPoints(24));
    });
    // client to master
    const cStart = new THREE.Vector3(...CLIENT_POS);
    const cEnd = new THREE.Vector3(...MASTER_POS);
    const cMid = cStart.clone().lerp(cEnd, 0.5);
    cMid.x += 0.8;
    pts.push(new THREE.QuadraticBezierCurve3(cStart, cMid, cEnd).getPoints(24));
    return pts;
  }, []);

  return (
    <>
      {lines.map((points, i) => (
        <line key={i}>
          <bufferGeometry>
            <bufferAttribute
              attach="attributes-position"
              count={points.length}
              array={new Float32Array(points.flatMap((p) => [p.x, p.y, p.z]))}
              itemSize={3}
            />
          </bufferGeometry>
          <lineBasicMaterial color="#2d3436" transparent opacity={0.35} />
        </line>
      ))}
    </>
  );
}

export function ParticleField() {
  const count = 400;
  const ref = useRef<THREE.Points>(null);
  const positions = useMemo(() => {
    const arr = new Float32Array(count * 3);
    for (let i = 0; i < count; i++) {
      arr[i * 3] = (Math.random() - 0.5) * 20;
      arr[i * 3 + 1] = (Math.random() - 0.5) * 12;
      arr[i * 3 + 2] = (Math.random() - 0.5) * 20;
    }
    return arr;
  }, []);

  useFrame(({ clock }) => {
    if (ref.current) {
      ref.current.rotation.y = clock.elapsedTime * 0.02;
    }
  });

  return (
    <points ref={ref}>
      <bufferGeometry>
        <bufferAttribute attach="attributes-position" count={count} array={positions} itemSize={3} />
      </bufferGeometry>
      <pointsMaterial size={0.04} color="#6c5ce7" transparent opacity={0.6} sizeAttenuation />
    </points>
  );
}

export function UploadBeam() {
  const phase = useStore((s) => s.phase);
  const ref = useRef<THREE.Mesh>(null);

  useFrame(({ clock }) => {
    if (ref.current && (phase === 'uploading' || phase === 'chunking')) {
      ref.current.scale.y = 0.5 + Math.sin(clock.elapsedTime * 6) * 0.2;
    }
  });

  if (phase !== 'uploading' && phase !== 'chunking') return null;

  const start = new THREE.Vector3(...CLIENT_POS);
  const end = new THREE.Vector3(...MASTER_POS);
  const mid = start.clone().add(end).multiplyScalar(0.5);
  const len = start.distanceTo(end);

  return (
    <mesh ref={ref} position={mid.toArray()} rotation={[0, 0, 0]}>
      <cylinderGeometry args={[0.03, 0.08, len, 8]} />
      <meshBasicMaterial color="#74b9ff" transparent opacity={0.5} />
    </mesh>
  );
}
