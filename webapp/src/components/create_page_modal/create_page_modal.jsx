import React from 'react';
import PropTypes from 'prop-types';
import {
    Button,
    Modal,
} from 'react-bootstrap';

import ConfluenceField from '../confluence_field';
import Validator from '../validator';

const initialState = {
    title: '',
    selectedSpace: null,
    selectedParentPage: null,
    parentPageQuery: '',
    spaces: [],
    parentPageOptions: [],
    error: '',
    saving: false,
    loadingSpaces: false,
    loadingParents: false,
};

const normalizePageOptions = (options) => {
    if (!Array.isArray(options)) {
        return [];
    }

    return options.map((option) => ({
        ...option,
        value: String(option?.value ?? ''),
        label: String(option?.label ?? option?.value ?? ''),
    }));
};

export default class CreatePageModal extends React.PureComponent {
    static propTypes = {
        modalState: PropTypes.object.isRequired,
        post: PropTypes.object,
        theme: PropTypes.object,
        close: PropTypes.func.isRequired,
        createPageFromPost: PropTypes.func.isRequired,
        getCreatePageSpaces: PropTypes.func.isRequired,
        searchCreatePageParents: PropTypes.func.isRequired,
    };

    static defaultProps = {
        post: null,
        theme: null,
    };

    constructor(props) {
        super(props);
        this.state = initialState;
        this.validator = new Validator();
        this.parentSearchRequest = 0;
    }

    componentDidUpdate(prevProps) {
        if (this.props.modalState.postId && this.props.modalState.postId !== prevProps.modalState.postId) {
            this.setState({
                ...initialState,
                title: this.getDefaultTitle(),
            }, this.loadSpaces);
        }
    }

    loadSpaces = async () => {
        this.setState({loadingSpaces: true});
        const response = await this.props.getCreatePageSpaces();
        const payload = response.data || {};
        const spaces = Array.isArray(payload.spaces) ? payload.spaces : [];
        const lastSelectedSpaceKey = payload.lastSelectedSpaceKey;
        const selectedSpace = spaces.find((space) => space.value === lastSelectedSpaceKey) || null;
        this.setState({
            loadingSpaces: false,
            spaces,
            selectedSpace,
            error: response.error ? (response.error.response?.text || 'Failed to load Confluence spaces.') : '',
        });
    };

    getDefaultTitle = () => {
        const message = this.props.post?.message || '';
        const firstLine = message.split('\n').find((line) => line.trim());
        if (!firstLine) {
            return 'Mattermost note';
        }

        return firstLine.trim().slice(0, 80);
    };

    handleClose = (e) => {
        if (e && e.preventDefault) {
            e.preventDefault();
        }

        this.parentSearchRequest += 1;
        this.setState(initialState, this.props.close);
    };

    handleChange = (key) => (e) => {
        this.setState({[key]: e.target.value});
    };

    handleSpaceChange = (selectedSpace) => {
        this.parentSearchRequest += 1;
        this.setState({
            selectedSpace,
            selectedParentPage: null,
            parentPageOptions: [],
            parentPageQuery: '',
            error: '',
        });
    };

    handleParentPageChange = (selectedParentPage) => {
        this.setState({selectedParentPage});
    };

    handleParentPageSearch = async (query, meta) => {
        if (meta?.action && meta.action !== 'input-change') {
            return query;
        }

        this.setState({parentPageQuery: query, error: ''});
        if (!this.state.selectedSpace || !query || query.trim().length < 2) {
            this.setState({
                parentPageOptions: [],
                loadingParents: false,
            });
            return query;
        }

        const requestID = this.parentSearchRequest + 1;
        this.parentSearchRequest = requestID;
        this.setState({loadingParents: true});
        const response = await this.props.searchCreatePageParents(this.state.selectedSpace.value, query.trim());
        if (requestID !== this.parentSearchRequest) {
            return query;
        }

        this.setState({
            loadingParents: false,
            parentPageOptions: normalizePageOptions(response.data),
            error: response.error ? (response.error.response?.text || 'Failed to search Confluence pages.') : '',
        });
        return query;
    };

    handleSubmit = async () => {
        if (!this.validator.validate()) {
            return;
        }

        this.setState({
            saving: true,
            error: '',
        });

        const response = await this.props.createPageFromPost({
            postID: this.props.modalState.postId,
            title: this.state.title.trim(),
            spaceKey: this.state.selectedSpace?.value,
            parentPageID: this.state.selectedParentPage?.value || '',
        });

        if (response.error) {
            this.setState({
                saving: false,
                error: response.error.response?.text || 'Failed to create Confluence page.',
            });
            return;
        }

        this.handleClose();
    };

    render() {
        const visible = Boolean(this.props.modalState.postId);
        const {saving, error, loadingSpaces, loadingParents} = this.state;
        const theme = this.props.theme || {};
        const modalStyles = {
            header: {
                color: theme.centerChannelColor,
                backgroundColor: theme.centerChannelBg,
            },
            body: {
                color: theme.centerChannelColor,
                backgroundColor: theme.centerChannelBg,
            },
            footer: {
                color: theme.centerChannelColor,
                backgroundColor: theme.centerChannelBg,
            },
        };

        return (
            <Modal
                show={visible}
                onHide={this.handleClose}
                backdrop={'static'}
            >
                <Modal.Header closeButton={true} style={modalStyles.header}>
                    <Modal.Title>{'Create Confluence Page'}</Modal.Title>
                </Modal.Header>
                <Modal.Body style={modalStyles.body}>
                    <ConfluenceField
                        label={'Title'}
                        type={'text'}
                        fieldType={'input'}
                        required={true}
                        placeholder={'Enter the Confluence page title.'}
                        value={this.state.title}
                        theme={theme}
                        addValidation={this.validator.addValidation}
                        removeValidation={this.validator.removeValidation}
                        onChange={this.handleChange('title')}
                    />
                    <ConfluenceField
                        label={'Space'}
                        fieldType={'dropDown'}
                        required={true}
                        placeholder={'Select a Confluence space.'}
                        value={this.state.selectedSpace}
                        options={this.state.spaces}
                        theme={theme}
                        isSearchable={true}
                        isMulti={false}
                        addValidation={this.validator.addValidation}
                        removeValidation={this.validator.removeValidation}
                        onChange={this.handleSpaceChange}
                        isLoading={loadingSpaces}
                        noOptionsMessage={() => loadingSpaces ? 'Loading spaces...' : 'No available spaces found.'}
                    />
                    <ConfluenceField
                        label={'Parent Page'}
                        fieldType={'dropDown'}
                        required={false}
                        placeholder={this.state.selectedSpace ? 'Search parent page by title.' : 'Select a space first.'}
                        value={this.state.selectedParentPage}
                        options={this.state.parentPageOptions}
                        theme={theme}
                        isSearchable={true}
                        isMulti={false}
                        addValidation={this.validator.addValidation}
                        removeValidation={this.validator.removeValidation}
                        onChange={this.handleParentPageChange}
                        onInputChange={this.handleParentPageSearch}
                        filterOption={() => true}
                        isDisabled={!this.state.selectedSpace}
                        isLoading={loadingParents}
                        noOptionsMessage={() => {
                            if (!this.state.selectedSpace) {
                                return 'Select a space first.';
                            }
                            if (this.state.parentPageQuery.trim().length < 2) {
                                return 'Type at least 2 characters to search.';
                            }
                            if (loadingParents) {
                                return 'Searching pages...';
                            }
                            return 'No matching pages found.';
                        }}
                    />
                    {Boolean(error) && (
                        <p className='alert alert-danger'>
                            <i className='fa fa-warning' title='Warning Icon'/>
                            <span> {error}</span>
                        </p>
                    )}
                </Modal.Body>
                <Modal.Footer style={modalStyles.footer}>
                    <Button
                        type='button'
                        bsStyle='link'
                        onClick={this.handleClose}
                    >
                        {'Cancel'}
                    </Button>
                    <Button
                        type='submit'
                        bsStyle='primary'
                        onClick={this.handleSubmit}
                        disabled={saving}
                    >
                        {saving && <span className='fa fa-spinner fa-fw fa-pulse spinner'/>}
                        {'Create Page'}
                    </Button>
                </Modal.Footer>
            </Modal>
        );
    }
}
