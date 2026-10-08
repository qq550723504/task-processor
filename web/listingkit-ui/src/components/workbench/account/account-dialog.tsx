"use client";
import { useEffect, useId, useRef } from "react";
import { Button } from "@/components/ui/button";
import styles from "./roles.module.css";

export function AccountDialog({ title, drawer = false, onClose, children }: { title: string; drawer?: boolean; onClose: () => void; children: React.ReactNode }) {
  const ref = useRef<HTMLDialogElement>(null), close = useRef<HTMLButtonElement>(null), id = useId();
  useEffect(() => {
    const element = ref.current, previous = document.activeElement;
    element?.showModal(); close.current?.focus();
    return () => { element?.close(); if (previous instanceof HTMLElement && previous.isConnected) previous.focus(); };
  }, []);
  return <dialog ref={ref} className={drawer ? styles.drawer : styles.modal} aria-labelledby={id} onCancel={e => { e.preventDefault(); onClose(); }}><header><h2 id={id}>{title}</h2><Button ref={close} variant="ghost" aria-label={`关闭${title}`} onClick={onClose}>×</Button></header>{children}</dialog>;
}
