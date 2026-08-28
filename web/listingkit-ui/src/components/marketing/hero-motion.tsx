"use client";

import Image from "next/image";
import { MotionConfig, motion } from "motion/react";

import styles from "./marketing-homepage.module.css";

const ASSET = "/sumi";
const LOOP = { duration: 24, repeat: Infinity } as const;

export function HeroMotionLayers() {
  return (
    <MotionConfig reducedMotion="user">
      <div className={styles.heroMotion} aria-hidden="true">
        <motion.div
          className={styles.heroAmbientGlow}
          data-node-id="356:303"
          initial={{ opacity: 0.34 }}
          animate={{ opacity: [0.34, 0.66, 0.46, 0.34] }}
          transition={{ opacity: { ...LOOP, times: [0, 0.3333, 0.6667, 1], ease: "easeInOut" } }}
        >
          <Image src={`${ASSET}/hero-background-glow.svg`} alt="" fill sizes="360px" />
        </motion.div>

        <FlowBand nodeId="356:304" className={styles.heroFlowOne} source="hero-flow-01.svg" initialOpacity={0.35} initialX={-14} opacity={[0.35, 0.82, 0.585, 0.35]} x={[-14, 18, -14]} />
        <FlowBand nodeId="356:306" className={styles.heroFlowTwo} source="hero-flow-02.svg" initialOpacity={0.24} initialX={18} opacity={[0.24, 0.64, 0.44, 0.24]} x={[18, -12, 18]} />
        <FlowBand nodeId="356:308" className={styles.heroFlowThree} source="hero-flow-03.svg" initialOpacity={0.18} initialX={-8} opacity={[0.18, 0.52, 0.35, 0.18]} x={[-8, 16, -8]} />

        <PulseNode nodeId="356:310" className={styles.heroPulseOne} source="hero-pulse-01.svg" times={[0, 0.125, 0.25, 1]} />
        <PulseNode nodeId="356:311" className={styles.heroPulseTwo} source="hero-pulse-02.svg" times={[0, 0.25, 0.375, 1]} />
        <PulseNode nodeId="356:312" className={styles.heroPulseThree} source="hero-pulse-03.svg" times={[0, 0.375, 0.5, 1]} />
      </div>
    </MotionConfig>
  );
}

function FlowBand({
  nodeId,
  className,
  source,
  initialOpacity,
  initialX,
  opacity,
  x,
}: {
  nodeId: string;
  className: string;
  source: string;
  initialOpacity: number;
  initialX: number;
  opacity: number[];
  x: number[];
}) {
  return (
    <motion.div
      className={className}
      data-node-id={nodeId}
      initial={{ opacity: initialOpacity, x: initialX }}
      animate={{ opacity, x }}
      transition={{
        opacity: { ...LOOP, times: [0, 0.25, 0.5833, 1], ease: "easeInOut" },
        x: { ...LOOP, times: [0, 0.5, 1], ease: "easeInOut" },
      }}
    >
      <Image src={`${ASSET}/${source}`} alt="" fill sizes="360px" />
    </motion.div>
  );
}

function PulseNode({
  nodeId,
  className,
  source,
  times,
}: {
  nodeId: string;
  className: string;
  source: string;
  times: number[];
}) {
  return (
    <motion.div
      className={className}
      data-node-id={nodeId}
      initial={{ opacity: 0.36, scaleX: 1, scaleY: 1 }}
      animate={{ opacity: [0.36, 1, 0.42, 0.36], scaleX: [1, 1.38, 1, 1], scaleY: [1, 1.38, 1, 1] }}
      transition={{
        opacity: { ...LOOP, times, ease: "easeInOut" },
        scaleX: { ...LOOP, times, ease: ["easeInOut", "easeInOut", "linear"] },
        scaleY: { ...LOOP, times, ease: ["easeInOut", "easeInOut", "linear"] },
      }}
    >
      <Image src={`${ASSET}/${source}`} alt="" fill sizes="36px" />
    </motion.div>
  );
}
