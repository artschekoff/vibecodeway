import { createContext, useContext, useState, useCallback, useRef, type ReactNode } from "react";

export interface ProgressState {
  isActive: boolean;
  title: string;
  /** Display labels for every step (one per item being processed). */
  items: string[];
  /** 0-based index of the step currently executing. -1 = not started yet. */
  currentIndex: number;
  /** 1-based count of completed steps. */
  current: number;
  total: number;
  stepText: string;
  /** Whether a cancel handler is registered for the current operation. */
  cancellable: boolean;
}

interface ProgressContextValue extends ProgressState {
  /** Start a new operation. items = ordered display labels for each step. */
  begin(title: string, items: string[], onCancel?: () => void): void;
  /** Advance to the next step, optionally overriding the step description. */
  next(stepText?: string): void;
  /** Mark the operation done and hide the modal. */
  done(): void;
  /** Directly set current index + step text (used for event-driven progress). */
  set(currentIndex: number, total: number, stepText: string): void;
  /** Invoke the cancel handler registered via begin(), if any. */
  cancel(): void;
}

const defaultState: ProgressState = {
  isActive: false,
  title: "",
  items: [],
  currentIndex: -1,
  current: 0,
  total: 0,
  stepText: "",
  cancellable: false,
};

const ProgressContext = createContext<ProgressContextValue>({
  ...defaultState,
  begin: () => {},
  next: () => {},
  done: () => {},
  set: () => {},
  cancel: () => {},
});

export function ProgressProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<ProgressState>(defaultState);
  const cancelRef = useRef<(() => void) | undefined>(undefined);

  const begin = useCallback((title: string, items: string[], onCancel?: () => void) => {
    cancelRef.current = onCancel;
    setState({
      isActive: true,
      title,
      items,
      currentIndex: -1,
      current: 0,
      total: items.length,
      stepText: "Starting…",
      cancellable: !!onCancel,
    });
  }, []);

  const next = useCallback((stepText?: string) => {
    setState((prev) => {
      const nextIndex = prev.currentIndex + 1;
      return {
        ...prev,
        currentIndex: nextIndex,
        current: nextIndex + 1,
        stepText: stepText ?? prev.items[nextIndex] ?? "",
      };
    });
  }, []);

  const set = useCallback((currentIndex: number, total: number, stepText: string) => {
    setState((prev) => ({
      ...prev,
      isActive: true,
      currentIndex,
      current: currentIndex + 1,
      total,
      stepText,
    }));
  }, []);

  const done = useCallback(() => {
    cancelRef.current = undefined;
    setState(defaultState);
  }, []);

  const cancel = useCallback(() => {
    cancelRef.current?.();
  }, []);

  return (
    <ProgressContext.Provider value={{ ...state, begin, next, done, set, cancel }}>
      {children}
    </ProgressContext.Provider>
  );
}

export function useProgress() {
  return useContext(ProgressContext);
}
