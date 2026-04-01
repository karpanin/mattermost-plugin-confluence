import React from 'react';
import ConfluenceIcon from './confluence_icon';

export default function CreatePageAction({actionText = 'Create Confluence page'}) {
    return (
        <>
            <ConfluenceIcon type='menu'/>
            {actionText}
        </>
    );
}
