import type { ReactNode } from "react";
import { Component } from "react";

type Props = {
  children: ReactNode;
};

type State = {
  error: Error | null;
};

export default class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error) {
    // eslint-disable-next-line no-console
    console.error("UI crashed:", error);
  }

  render() {
    if (this.state.error) {
      return (
        <div style={{ padding: 24 }}>
          <h2 style={{ marginTop: 0 }}>UI error</h2>
          <p>The dashboard hit a runtime error.</p>
          <pre
            style={{
              background: "#161d27",
              border: "1px solid #2a3441",
              borderRadius: 10,
              padding: 12,
              overflow: "auto",
            }}
          >
            {this.state.error.message}
          </pre>
          <p style={{ opacity: 0.8 }}>
            Open DevTools → Console to see the stack trace.
          </p>
        </div>
      );
    }

    return this.props.children;
  }
}

