import React, {useState, useEffect, useCallback} from 'react';
import * as api from '../actions/api';
import ResourceCard from './ResourceCard';
import BookingModal from './BookingModal';
import AdminPanel from './AdminPanel';
import HistoryPanel from './HistoryPanel';

interface Props {
    theme: any;
}

const RHSView: React.FC<Props> = ({theme}) => {
    const [statuses, setStatuses] = useState<any[]>([]);
    const [isAdmin, setIsAdmin] = useState(false);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState('');
    const [view, setView] = useState<'list' | 'admin' | 'history'>('list');
    const [modal, setModal] = useState<{resourceId: string; mode: 'book' | 'queue' | 'extend'} | null>(null);
    const [historyResourceId, setHistoryResourceId] = useState('');

    const refresh = useCallback(async () => {
        try {
            const data = await api.getAllStatus();
            setStatuses(data.statuses || []);
            setIsAdmin(data.is_admin || false);
            setError('');
        } catch (e: any) {
            setError(e.message);
        } finally {
            setLoading(false);
        }
    }, []);

    useEffect(() => {
        refresh();
        const interval = setInterval(refresh, 10000);
        return () => clearInterval(interval);
    }, [refresh]);

    const styles = getStyles(theme);

    if (view === 'admin') {
        return (
            <div style={styles.container}>
                <div style={styles.header}>
                    <button style={styles.backBtn} onClick={() => {setView('list'); refresh();}}>← Назад</button>
                    <span style={styles.title}>Управление ресурсами</span>
                </div>
                <AdminPanel theme={theme} onBack={() => {setView('list'); refresh();}} />
            </div>
        );
    }

    if (view === 'history') {
        return (
            <div style={styles.container}>
                <div style={styles.header}>
                    <button style={styles.backBtn} onClick={() => setView('list')}>← Назад</button>
                    <span style={styles.title}>История</span>
                </div>
                <HistoryPanel resourceId={historyResourceId} theme={theme} />
            </div>
        );
    }

    return (
        <div style={styles.container}>
            <div style={styles.header}>
                <span style={styles.title}>🖥️ Ресурсы</span>
                <div>
                    <button style={styles.headerBtn} onClick={refresh} title="Обновить">🔄</button>
                    {isAdmin && <button style={styles.headerBtn} onClick={() => setView('admin')} title="Управление">⚙️</button>}
                </div>
            </div>
            {error && <div style={styles.error}>{error}</div>}
            {loading && <div style={styles.loading}>Загрузка...</div>}
            {!loading && statuses.length === 0 && (
                <div style={styles.empty}>
                    Ресурсы не настроены.{isAdmin ? ' Нажмите ⚙️ для добавления.' : ' Обратитесь к администратору.'}
                </div>
            )}
            {statuses.map((status: any) => (
                <ResourceCard
                    key={status.resource.id}
                    status={status}
                    theme={theme}
                    isAdmin={isAdmin}
                    onBook={() => setModal({resourceId: status.resource.id, mode: 'book'})}
                    onQueue={() => setModal({resourceId: status.resource.id, mode: 'queue'})}
                    onExtend={() => setModal({resourceId: status.resource.id, mode: 'extend'})}
                    onRelease={async () => {
                        try { await api.releaseResource(status.resource.id); refresh(); }
                        catch (e: any) { alert(e.message); }
                    }}
                    onLeaveQueue={async () => {
                        try { await api.leaveQueue(status.resource.id); refresh(); }
                        catch (e: any) { alert(e.message); }
                    }}
                    onSubscribe={async () => {
                        try { await api.subscribeResource(status.resource.id); refresh(); }
                        catch (e: any) { alert(e.message); }
                    }}
                    onUnsubscribe={async () => {
                        try { await api.unsubscribeResource(status.resource.id); refresh(); }
                        catch (e: any) { alert(e.message); }
                    }}
                    onHistory={() => {
                        setHistoryResourceId(status.resource.id);
                        setView('history');
                    }}
                />
            ))}
            {modal && (
                <BookingModal
                    resourceId={modal.resourceId}
                    mode={modal.mode}
                    theme={theme}
                    onClose={() => setModal(null)}
                    onDone={() => { setModal(null); refresh(); }}
                />
            )}
        </div>
    );
};

function getStyles(theme: any) {
    return {
        container: {
            padding: '12px', height: '100%', overflowY: 'auto' as const,
            backgroundColor: theme?.centerChannelBg || '#fff',
            color: theme?.centerChannelColor || '#333',
        },
        header: {
            display: 'flex', justifyContent: 'space-between', alignItems: 'center',
            marginBottom: '12px', paddingBottom: '8px',
            borderBottom: `1px solid ${theme?.centerChannelColor ? theme.centerChannelColor + '22' : '#ddd'}`,
        },
        title: { fontSize: '16px', fontWeight: 600 as const },
        headerBtn: {
            background: 'none', border: 'none', cursor: 'pointer',
            fontSize: '16px', padding: '4px 8px', borderRadius: '4px',
            color: theme?.centerChannelColor || '#333',
        },
        backBtn: {
            background: 'none', border: 'none', cursor: 'pointer',
            fontSize: '13px', padding: '4px 8px',
            color: theme?.linkColor || '#2389d7',
        },
        error: {
            padding: '8px', backgroundColor: '#ffebee', color: '#c62828',
            borderRadius: '4px', marginBottom: '8px', fontSize: '13px',
        },
        loading: { textAlign: 'center' as const, padding: '20px', color: theme?.centerChannelColor || '#666' },
        empty: {
            textAlign: 'center' as const, padding: '20px',
            color: theme?.centerChannelColor ? theme.centerChannelColor + '88' : '#999',
            fontSize: '13px',
        },
    };
}

export default RHSView;
