import { useState } from "react";
import { ChevronDown, ChevronRight, Check, X } from "lucide-react";
import { AnswerOpenQuestion, DismissOpenQuestion } from "../../wailsjs/go/main/App";
import type { OpenQuestion } from "../types";

interface Props {
  questions: OpenQuestion[];
  onUpdated: () => void;
}

export default function OpenQuestionsPanel({ questions, onUpdated }: Props) {
  const [expanded, setExpanded] = useState(true);
  const [answeringId, setAnsweringId] = useState<string | null>(null);
  const [answerText, setAnswerText] = useState("");

  const open      = questions.filter((q) => q.status === "open");
  const answered  = questions.filter((q) => q.status === "answered");
  const dismissed = questions.filter((q) => q.status === "dismissed");

  if (questions.length === 0) return null;

  const handleAnswer = async (id: string) => {
    if (!answerText.trim()) return;
    await AnswerOpenQuestion(id, answerText.trim());
    setAnsweringId(null);
    setAnswerText("");
    onUpdated();
  };

  const handleDismiss = async (id: string) => {
    await DismissOpenQuestion(id);
    onUpdated();
  };

  return (
    <section className="open-questions-panel">
      <button className="open-questions-header" onClick={() => setExpanded((x) => !x)}>
        {expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        <span>Open Questions</span>
        {open.length > 0 && <span className="open-questions-count">{open.length} open</span>}
      </button>

      {expanded && (
        <div className="open-questions-list">
          {open.map((q) => (
            <div key={q.id} className="open-question-item open-question-item--open">
              <div className="open-question-text">{q.question}</div>
              {answeringId === q.id ? (
                <div className="open-question-answer-form">
                  <input autoFocus type="text" placeholder="Enter answer…" value={answerText}
                    onChange={(e) => setAnswerText(e.target.value)}
                    onKeyDown={(e) => e.key === "Enter" && handleAnswer(q.id)}
                    className="open-question-answer-input" />
                  <button onClick={() => handleAnswer(q.id)} className="btn-icon" title="Submit"><Check size={13} /></button>
                  <button onClick={() => setAnsweringId(null)} className="btn-icon" title="Cancel"><X size={13} /></button>
                </div>
              ) : (
                <div className="open-question-actions">
                  <button className="open-question-action-btn" onClick={() => { setAnsweringId(q.id); setAnswerText(""); }}>Answer</button>
                  <button className="open-question-action-btn open-question-action-btn--dismiss" onClick={() => handleDismiss(q.id)}>Dismiss</button>
                </div>
              )}
            </div>
          ))}
          {answered.length > 0 && (
            <details className="open-questions-resolved">
              <summary>Answered ({answered.length})</summary>
              {answered.map((q) => (
                <div key={q.id} className="open-question-item open-question-item--answered">
                  <div className="open-question-text">{q.question}</div>
                  <div className="open-question-answer">{q.answer}</div>
                </div>
              ))}
            </details>
          )}
          {dismissed.length > 0 && (
            <details className="open-questions-resolved">
              <summary>Dismissed ({dismissed.length})</summary>
              {dismissed.map((q) => (
                <div key={q.id} className="open-question-item open-question-item--dismissed">
                  <div className="open-question-text">{q.question}</div>
                </div>
              ))}
            </details>
          )}
        </div>
      )}
    </section>
  );
}
