import { Suspense } from 'react';
import { Canvas } from '@react-three/fiber';
import { OrbitControls, Stars, Environment } from '@react-three/drei';
import { EffectComposer, Bloom, Vignette } from '@react-three/postprocessing';
import {
  MasterNode,
  StorageCluster,
  ClientUploader,
  ChunkOrbit,
  DataPackets,
  ConnectionLines,
  ParticleField,
  UploadBeam,
} from './ClusterScene';

function Scene() {
  return (
    <>
      <color attach="background" args={['#0a0a12']} />
      <fog attach="fog" args={['#0a0a12', 12, 28]} />
      <ambientLight intensity={0.25} />
      <directionalLight position={[8, 12, 6]} intensity={1.2} castShadow />
      <pointLight position={[0, 8, 0]} intensity={0.6} color="#6c5ce7" />

      <Stars radius={80} depth={40} count={3000} factor={3} saturation={0} fade speed={0.5} />
      <ParticleField />
      <ConnectionLines />
      <UploadBeam />

      <ClientUploader />
      <MasterNode />
      <StorageCluster />
      <ChunkOrbit />
      <DataPackets />

      <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, -2, 0]} receiveShadow>
        <circleGeometry args={[12, 64]} />
        <meshStandardMaterial color="#12121f" metalness={0.9} roughness={0.3} />
      </mesh>

      <OrbitControls
        enablePan={false}
        minDistance={8}
        maxDistance={22}
        maxPolarAngle={Math.PI / 2.1}
        autoRotate
        autoRotateSpeed={0.3}
      />
      <Environment preset="night" />

      <EffectComposer>
        <Bloom luminanceThreshold={0.2} luminanceSmoothing={0.9} intensity={1.2} />
        <Vignette eskil={false} offset={0.2} darkness={0.7} />
      </EffectComposer>
    </>
  );
}

export function SceneCanvas() {
  return (
    <Canvas
      shadows
      camera={{ position: [0, 6, 14], fov: 50 }}
      gl={{ antialias: true, alpha: true }}
      dpr={[1, 2]}
    >
      <Suspense fallback={null}>
        <Scene />
      </Suspense>
    </Canvas>
  );
}
