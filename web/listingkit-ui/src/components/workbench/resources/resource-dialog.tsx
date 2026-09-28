"use client";
import { useEffect, useRef, type ReactNode } from "react";
import { Button } from "@/components/ui/button";
import styles from "./resources.module.css";
export function ResourceDialog({
  title,
  onClose,
  locked = false,
  children,
}: {
  title: string;
  onClose: () => void;
  locked?: boolean;
  children: ReactNode;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const dialog = ref.current;
    dialog?.showModal();
    return () => dialog?.close();
  }, []);
  return (
    <dialog
      ref={ref}
      className={styles.resourceDialog}
      aria-label={title}
      onCancel={(event) => {
        event.preventDefault();
        if (!locked) onClose();
      }}
    >
      <div className={styles.dialogHeading}>
        <h2>{title}</h2>
        <Button
          variant="outline"
          disabled={locked}
          onClick={onClose}
          aria-label="关闭"
        >
          ×
        </Button>
      </div>
      {children}
    </dialog>
  );
}
