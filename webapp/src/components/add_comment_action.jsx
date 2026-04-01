import React from 'react';
import ConfluenceIcon from './confluence_icon';

export default function AddCommentAction({actionText = 'Add comment to Confluence page'}) {
    return (
        <>
            <ConfluenceIcon type='menu'/>
            {actionText}
        </>
    );
}
