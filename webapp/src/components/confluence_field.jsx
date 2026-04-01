import React from 'react';
import PropTypes from 'prop-types';
import {ControlLabel, FormControl, FormGroup} from 'react-bootstrap';
import Select from 'react-select';

import {getStyleForReactSelect} from '../utils/react_select_styles';

export default class ConfluenceField extends React.PureComponent {
    static propTypes = {
        required: PropTypes.bool.isRequired,
        value: PropTypes.oneOfType([
            PropTypes.string,
            PropTypes.number,
            PropTypes.object,
            PropTypes.array,
        ]),
        label: PropTypes.string.isRequired,
        onChange: PropTypes.func.isRequired,
        addValidation: PropTypes.func.isRequired,
        removeValidation: PropTypes.func.isRequired,
        theme: PropTypes.object,
        fieldType: PropTypes.string.isRequired,
        readOnly: PropTypes.bool,
        formGroupStyle: PropTypes.object,
        formControlStyle: PropTypes.object,
        type: PropTypes.string,
        placeholder: PropTypes.string,
        name: PropTypes.string,
        options: PropTypes.array,
        isMulti: PropTypes.bool,
        isSearchable: PropTypes.bool,
        testId: PropTypes.string,
        onInputChange: PropTypes.func,
        isLoading: PropTypes.bool,
        isDisabled: PropTypes.bool,
        noOptionsMessage: PropTypes.func,
        disableClientFilter: PropTypes.bool,
    };

    static defaultProps = {
        readOnly: false,
        formGroupStyle: {},
        formControlStyle: {},
        onInputChange: null,
        isLoading: false,
        isDisabled: false,
        noOptionsMessage: undefined,
        disableClientFilter: false,
    };

    constructor(props) {
        super(props);
        this.state = {
            valid: true,
        };
    }

    componentDidMount() {
        if (this.props.addValidation) {
            this.props.addValidation(this.isValid);
        }
    }

    handleChange = (e) => {
        if (!this.state.valid) {
            this.setState({
                valid: true,
            });
        }
        this.props.onChange(e);
    };

    componentWillUnmount() {
        if (this.props.removeValidation) {
            this.props.removeValidation(this.isValid);
        }
    }

    isValid = () => {
        const {fieldType, value, required} = this.props;
        if (required &&
            (value === null ||
            (typeof value === 'string' && !value.trim()) ||
            (fieldType === 'dropDown' && value.length === 0) ||
            !value)
        ) {
            this.setState({
                valid: false,
            });
            return false;
        }
        return true;
    };

    render() {
        const {
            required, fieldType, theme, label, formGroupStyle, formControlStyle,
            value, type, placeholder, name, readOnly, options, isMulti, isSearchable, testId, onInputChange, isLoading, isDisabled,
            noOptionsMessage, disableClientFilter,
        } = this.props;
        const requiredErrorMsg = 'This field is required.';
        let requiredError = null;
        if (required && !this.state.valid) {
            requiredError = (
                <p className='help-text error-text'>
                    <span>{requiredErrorMsg}</span>
                </p>
            );
        }
        let field = null;
        const normalizeOption = (option) => {
            if (!option || typeof option !== 'object') {
                const normalized = String(option ?? '');
                return {value: normalized, label: normalized};
            }

            return {
                ...option,
                value: String(option.value ?? ''),
                label: String(option.label ?? option.value ?? ''),
            };
        };

        const normalizedOptions = Array.isArray(options) ? options.map(normalizeOption) : [];
        const normalizedValue = Array.isArray(value) ? value.map(normalizeOption) : (value && typeof value === 'object' ? normalizeOption(value) : value);

        if (fieldType === 'input') {
            field = (
                <FormControl
                    style={formControlStyle}
                    type={type}
                    placeholder={placeholder}
                    value={value}
                    readOnly={readOnly}
                    onChange={this.handleChange}
                    data-testid={testId}
                />
            );
        } else if (fieldType === 'dropDown') {
            field = (
                <Select
                    name={name}
                    value={normalizedValue}
                    options={normalizedOptions}
                    isMulti={isMulti}
                    isSearchable={isSearchable}
                    menuPortalTarget={document.body}
                    menuPlacement='auto'
                    styles={getStyleForReactSelect(theme)}
                    onChange={this.handleChange}
                    onInputChange={onInputChange}
                    isLoading={isLoading}
                    isDisabled={isDisabled}
                    noOptionsMessage={noOptionsMessage}
                    inputId={testId}
                    placeholder={placeholder}
                    getOptionLabel={(option) => String(option?.label ?? '')}
                    getOptionValue={(option) => String(option?.value ?? '')}
                    filterOption={disableClientFilter ? () => true : undefined}
                />
            );
        }
        return (
            <FormGroup style={formGroupStyle}>
                <ControlLabel>{label}</ControlLabel>
                {required &&
                <span
                    className='error-text'
                    style={{marginLeft: '3px'}}
                >
                    {'*'}
                </span> }
                {field}
                {requiredError}
            </FormGroup>
        );
    }
}
