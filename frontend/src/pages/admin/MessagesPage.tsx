import { useEffect, useState } from 'react';
import { useOutletContext } from 'react-router-dom';
import { Mail, CheckCircle } from 'lucide-react';
import { toast } from 'sonner';
import { messagesApi } from '../../api/client';
import type { Message } from '../../types';
import LoadingSpinner from '../../components/LoadingSpinner';
import { getErrorMessage } from '../../utils/errors';

export default function MessagesPage() {
  const { setUnreadCount } = useOutletContext<{ setUnreadCount: React.Dispatch<React.SetStateAction<number>> }>() ?? {};
  const [messages, setMessages] = useState<Message[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    messagesApi.list()
      .then((data) => setMessages(data.messages))
      .catch(err => toast.error(getErrorMessage(err)))
      .finally(() => setLoading(false));
  }, []);

  const markAsRead = async (msg: Message) => {
    if (msg.read) return;
    try {
      await messagesApi.markRead(msg.id);
      setMessages(messages.map(m =>
        m.id === msg.id ? { ...m, read: true } : m
      ));
      setUnreadCount?.((prev) => Math.max(0, prev - 1));
    } catch {
      // ignore
    }
  };

  if (loading) return <LoadingSpinner size="lg" className="py-20" />;

  return (
    <div>
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-dark-50 flex items-center gap-3">
            <Mail className="w-7 h-7 text-primary-400" />
            Messages
          </h1>
          <p className="text-dark-400 mt-1">{messages.length} messages</p>
        </div>
      </div>

      {messages.length === 0 ? (
        <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-12 text-center">
          <Mail className="w-12 h-12 text-dark-600 mx-auto mb-4" />
          <p className="text-dark-400">No messages yet</p>
        </div>
      ) : (
        <div className="space-y-3">
          {messages.map((msg) => (
            <div
              key={msg.id}
              onClick={() => markAsRead(msg)}
              className={`bg-dark-900/50 border rounded-2xl p-6 cursor-pointer transition-colors ${
                msg.read
                  ? 'border-dark-800 opacity-70'
                  : 'border-primary-500/30 hover:border-primary-500/50'
              }`}
            >
              <div className="flex items-start justify-between">
                <div className="flex-1">
                  <div className="flex items-center gap-2">
                    <h3 className={`text-sm font-medium ${
                      msg.read ? 'text-dark-300' : 'text-dark-50'
                    }`}>
                      {msg.subject}
                    </h3>
                    {msg.isSystem && (
                      <span className="text-xs px-2 py-0.5 rounded-full bg-primary-500/10 text-primary-400">
                        System
                      </span>
                    )}
                  </div>
                  <p className="text-sm text-dark-400 mt-2 whitespace-pre-wrap">
                    {msg.body}
                  </p>
                  <p className="text-xs text-dark-500 mt-3">
                    {new Date(msg.createdAt).toLocaleString()}
                  </p>
                </div>
                {msg.read && (
                  <CheckCircle className="w-4 h-4 text-dark-600 flex-shrink-0 ml-4" />
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
