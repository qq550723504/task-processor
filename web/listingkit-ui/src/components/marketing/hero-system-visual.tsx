"use client";

import { useEffect, useState } from "react";
import {
  Bot,
  Boxes,
  BrainCircuit,
  Network,
  PackageSearch,
  Store,
  type LucideIcon,
} from "lucide-react";
import {
  MotionConfig,
  motion,
  useReducedMotion,
  type Variants,
} from "motion/react";

import styles from "./marketing-hero.module.css";

type CapabilityPosition =
  | "nodeControl"
  | "nodeAgent"
  | "nodeProduct"
  | "nodeTools"
  | "nodeListing"
  | "nodeConnectors";

type Capability = {
  key: string;
  eyebrow: string;
  label: string;
  icon: LucideIcon;
  position: CapabilityPosition;
};

const CAPABILITIES = [
  {
    key: "ai-control",
    eyebrow: "AI CONTROL",
    label: "模型与调用治理",
    icon: BrainCircuit,
    position: "nodeControl",
  },
  {
    key: "agent-runtime",
    eyebrow: "AGENT RUNTIME",
    label: "智能体运行时",
    icon: Bot,
    position: "nodeAgent",
  },
  {
    key: "product-intelligence",
    eyebrow: "PRODUCT INTEL",
    label: "商品智能",
    icon: PackageSearch,
    position: "nodeProduct",
  },
  {
    key: "commerce-tools",
    eyebrow: "COMMERCE TOOLS",
    label: "电商工具",
    icon: Boxes,
    position: "nodeTools",
  },
  {
    key: "listingkit",
    eyebrow: "LISTINGKIT",
    label: "上架执行面",
    icon: Store,
    position: "nodeListing",
  },
  {
    key: "connectors",
    eyebrow: "CONNECTORS",
    label: "平台连接器",
    icon: Network,
    position: "nodeConnectors",
  },
] as const satisfies readonly Capability[];

const CONNECTIONS = [
  { x2: 125, y2: 118 },
  { x2: 525, y2: 118 },
  { x2: 84, y2: 325 },
  { x2: 566, y2: 325 },
  { x2: 139, y2: 544 },
  { x2: 519, y2: 534 },
] as const;

const PARTICLES = [
  { left: "25.3%", top: "26.2%" },
  { left: "73.1%", top: "25.6%" },
  { left: "22.2%", top: "49.5%" },
  { left: "76.9%", top: "49.5%" },
  { left: "27.3%", top: "74%" },
  { left: "71.7%", top: "72.8%" },
] as const;

const coreVariants: Variants = {
  boot: { opacity: 0, scale: 0.55 },
  active: {
    opacity: 1,
    scale: 1,
    transition: { delay: 0.08, duration: 0.62, ease: "easeOut" },
  },
};

const ringVariants: Variants = {
  boot: { opacity: 0, scale: 0.76 },
  active: (index: number) => ({
    opacity: 1,
    scale: 1,
    transition: {
      delay: 0.22 + index * 0.12,
      duration: 0.82,
      ease: "easeOut",
    },
  }),
};

const connectorVariants: Variants = {
  boot: { opacity: 0, pathLength: 0 },
  active: (index: number) => ({
    opacity: 0.72,
    pathLength: 1,
    transition: {
      delay: 0.72 + index * 0.055,
      duration: 0.72,
      ease: "easeInOut",
    },
  }),
};

const capabilityVariants: Variants = {
  boot: { opacity: 0, scale: 0.82, y: 12 },
  active: (index: number) => ({
    opacity: 1,
    scale: 1,
    y: 0,
    transition: {
      delay: 0.92 + index * 0.105,
      duration: 0.46,
      ease: "easeOut",
    },
  }),
};

const particleVariants: Variants = {
  boot: { opacity: 0, scale: 0.4 },
  active: (index: number) => ({
    opacity: 1,
    scale: 1,
    transition: {
      delay: 1.34 + index * 0.075,
      duration: 0.35,
      ease: "easeOut",
    },
  }),
};

export function HeroSystemVisual() {
  const reduceMotion = Boolean(useReducedMotion());
  const [visualState, setVisualState] = useState<"active" | "boot">("active");

  useEffect(() => {
    if (reduceMotion) {
      return;
    }

    setVisualState("boot");
    const frame = window.requestAnimationFrame(() => setVisualState("active"));

    return () => window.cancelAnimationFrame(frame);
  }, [reduceMotion]);

  return (
    <MotionConfig reducedMotion="user">
      <motion.div
        animate={visualState}
        aria-label="硕米 AI 电商能力架构"
        className={styles.systemVisual}
        data-motion-sequence="boot-reveal-active-pulse"
        data-node-id="10:144"
        initial={false}
        role="group"
      >
        <div aria-hidden="true" className={styles.visualGlow} />

        <OrbitRing
          className={styles.ringOuter}
          direction={360}
          index={0}
          reduceMotion={reduceMotion}
        />
        <OrbitRing
          className={styles.ringMid}
          direction={-360}
          index={1}
          reduceMotion={reduceMotion}
        />
        <OrbitRing
          className={styles.ringInner}
          direction={360}
          index={2}
          reduceMotion={reduceMotion}
        />

        <svg
          aria-hidden="true"
          className={styles.connectors}
          focusable="false"
          viewBox="0 0 650 650"
        >
          <defs>
            <linearGradient id="hero-connection-gradient" x1="0" x2="1">
              <stop offset="0" stopColor="#4f7fff" stopOpacity="0.16" />
              <stop offset="0.55" stopColor="#5a8dff" stopOpacity="0.72" />
              <stop offset="1" stopColor="#8056ff" stopOpacity="0.24" />
            </linearGradient>
          </defs>
          {CONNECTIONS.map((connection, index) => (
            <motion.line
              custom={index}
              key={`${connection.x2}-${connection.y2}`}
              stroke="url(#hero-connection-gradient)"
              strokeWidth="1"
              variants={connectorVariants}
              x1="325"
              x2={connection.x2}
              y1="325"
              y2={connection.y2}
            />
          ))}
        </svg>

        <motion.div
          className={styles.coreAnchor}
          variants={coreVariants}
        >
          <div className={styles.core}>
            <span className={styles.coreOrb}>
              <motion.span
                animate={
                  reduceMotion
                    ? { opacity: 0.9, scale: 1 }
                    : {
                        opacity: [0.72, 1, 0.72],
                        scale: [1, 1.28, 1],
                      }
                }
                className={styles.coreOrbGlow}
                transition={
                  reduceMotion
                    ? { duration: 0 }
                    : {
                        delay: 2.15,
                        duration: 2.4,
                        ease: "easeInOut",
                        repeat: Infinity,
                        repeatDelay: 5.4,
                      }
                }
              />
            </span>
            <strong>SHUOMI</strong>
            <span>AI COMMERCE CORE</span>
          </div>
        </motion.div>

        <motion.ul
          aria-label="电商能力节点"
          className={styles.capabilityList}
        >
          {CAPABILITIES.map((capability, index) => {
            const Icon = capability.icon;
            return (
              <motion.li
                className={`${styles.capabilityCard} ${styles[capability.position]}`}
                custom={index}
                key={capability.key}
                variants={capabilityVariants}
              >
                <span aria-hidden="true" className={styles.capabilityIcon}>
                  <Icon size={16} strokeWidth={1.8} />
                </span>
                <span className={styles.capabilityCopy}>
                  <strong>{capability.eyebrow}</strong>
                  <span>{capability.label}</span>
                </span>
              </motion.li>
            );
          })}
        </motion.ul>

        {PARTICLES.map((particle, index) => (
          <motion.span
            aria-hidden="true"
            className={styles.particle}
            custom={index}
            key={`${particle.left}-${particle.top}`}
            style={particle}
            variants={particleVariants}
          />
        ))}
      </motion.div>
    </MotionConfig>
  );
}

function OrbitRing({
  className,
  direction,
  index,
  reduceMotion,
}: {
  className: string;
  direction: number;
  index: number;
  reduceMotion: boolean;
}) {
  return (
    <motion.div
      aria-hidden="true"
      className={`${styles.ringLayer} ${className}`}
      custom={index}
      variants={ringVariants}
    >
      <motion.span
        animate={{ rotate: reduceMotion ? 0 : direction }}
        className={styles.ringStroke}
        transition={
          reduceMotion
            ? { duration: 0 }
            : {
                delay: 1.7,
                duration: index === 0 ? 38 : index === 1 ? 31 : 24,
                ease: "linear",
                repeat: Infinity,
              }
        }
      />
    </motion.div>
  );
}
