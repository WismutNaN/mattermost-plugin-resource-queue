import React, {useState, useEffect} from 'react';
import * as api from '../actions/api';

interface Props {
    resourceId: string;
    theme: any;
}

const HistoryPanel: React.FC<Props> = ({resourceId, theme}) => {
    const [entries, setEntries] = useState<any[]>([]);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        api.getHistory(resourceId).then((data) => {
            setEntries(data || []);
        }).catch(() => {}).finally(() => setLoading(false));
    }, [resourceId]);

    const styles = getStyles(theme);

    if (loading) return <div style={styles.loading}>Загрузка...</div>;
    if (entries.length === 0) return <div style={styles.empty}>История пуста</div>;

    // Stats
    const totalMinutes = entries.reduce((acc: number, e: any) => {
        const dur = (new Date(e.ended_at).getTime() - new Date(e.started_at).getTime()) / 60000;
        return acc + dur;
    }, 0);

    const userStats: Record<string, number> = {};
    entries.forEach((e: any) => {
        const dur = (new Date(e.ended_at).getTime() - new Date(e.started_at).getTime()) / 60000;
        userStats[e.username || e.user_id] = (userStats[e.username || e.user_id] || 0) + dur;
    });

    const topUsers = Object.entries(userStats)
        .sort(([, a], [, b]) => (b as number) - (a as number))
        .slice(0, 5);

    return (
        <div>
            <div style={styles.stats}>
                <div style={styles.stat}>
                    <div style={styles.statValue}>{entries.length}</div>
                    <div style={styles.statLabel}>Сессий</div>
                </div>
                <div style={styles.stat}>
                    <div style={styles.statValue}>{formatMinutes(totalMinutes)}</div>
                    <div style={styles.statLabel}>Всего</div>
                </div>
                <div style={styles.stat}>
                    <div style={styles.statValue}>{Object.keys(userStats).length}</div>
                    <div style={styles.statLabel}>Пользователей</div>
                </div>
            </div>

            {topUsers.length > 0 && (
                <div style={styles.section}>
                    <div style={styles.sectionTitle}>Топ пользователей</div>
                    {topUsers.map(([uid, mins]) => (
                        <div key={uid} style={styles.topUserRow}>
                            <span>@{uid.substring(0, 8)}</span>
                            <span>{formatMinutes(mins as number)}</span>
                        </div>
                    ))}
                </div>
            )}

            <div style={styles.section}>
                <div style={styles.sectionTitle}>Последние сессии</div>
                {entries.slice(0, 30).map((e: any, i: number) => {
                    const dur = (new Date(e.ended_at).getTime() - new Date(e.started_at).getTime()) / 60000;
                    return (
                        <div key={i} style={styles.historyRow}>
                            <div style={styles.historyUser}>@{e.username || e.user_id}</div>
                            <div style={styles.historyTime}>
                                {new Date(e.started_at).toLocaleDateString()} {new Date(e.started_at).toLocaleTimeString([], {hour: '2-digit', minute: '2-digit'})}
                            </div>
                            <div style={styles.historyDur}>{formatMinutes(dur)}</div>
                            {e.purpose && <div style={styles.historyPurpose}>{e.purpose}</div>}
                        </div>
                    );
                })}
            </div>
        </div>
    );
};

function formatMinutes(m: number): string {
    if (m < 60) return `${Math.round(m)}м`;
    const h = Math.floor(m / 60);
    const mins = Math.round(m % 60);
    return mins > 0 ? `${h}ч ${mins}м` : `${h}ч`;
}

function getStyles(theme: any) {
    return {
        loading: {textAlign: 'center' as const, padding: '20px', fontSize: '13px'},
        empty: {textAlign: 'center' as const, padding: '20px', fontSize: '13px', color: '#999'},
        stats: {display: 'flex', gap: '8px', marginBottom: '12px'},
        stat: {
            flex: 1, textAlign: 'center' as const, padding: '10px',
            border: `1px solid ${theme?.centerChannelColor ? theme.centerChannelColor + '22' : '#eee'}`,
            borderRadius: '6px',
        },
        statValue: {fontSize: '18px', fontWeight: 700 as const},
        statLabel: {fontSize: '11px', color: theme?.centerChannelColor ? theme.centerChannelColor + '88' : '#888'},
        section: {marginBottom: '12px'},
        sectionTitle: {fontSize: '13px', fontWeight: 600 as const, marginBottom: '6px'},
        topUserRow: {
            display: 'flex', justifyContent: 'space-between', padding: '4px 8px',
            fontSize: '12px', borderBottom: `1px solid ${theme?.centerChannelColor ? theme.centerChannelColor + '11' : '#f5f5f5'}`,
        },
        historyRow: {
            padding: '6px 8px', fontSize: '12px',
            borderBottom: `1px solid ${theme?.centerChannelColor ? theme.centerChannelColor + '11' : '#f5f5f5'}`,
        },
        historyUser: {fontWeight: 500 as const},
        historyTime: {fontSize: '11px', color: theme?.centerChannelColor ? theme.centerChannelColor + '88' : '#888'},
        historyDur: {fontSize: '11px'},
        historyPurpose: {fontSize: '11px', fontStyle: 'italic' as const, color: theme?.centerChannelColor ? theme.centerChannelColor + 'aa' : '#666'},
    };
}

export default HistoryPanel;
