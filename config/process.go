package config

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"regexp"
	"sort"
	"time"

	"github.com/getblank/blank-sr/utils/array"

	log "github.com/Sirupsen/logrus"
	"github.com/imdario/mergo"
)

var (
	mustacheRgx    = regexp.MustCompile(`(?U)({{.+}})`)
	handleBarseRgx = regexp.MustCompile(`{?{{\s*(\w*)\s?(\w*)?\s?.*}}`)
	itemPropsRgx   = regexp.MustCompile(`\$item.([A-Za-z][A-Za-z0-9]*)`)
	actionIdRgx    = regexp.MustCompile(`^[A-Za-z_]+[A-Za-z0-9_]*$`)

	storeUpdateHandlers = []func(map[string]Store){}
)

func Init(confFile string) {
	makeDefaultSettings()
	readConfig(confFile)
}

func ReloadConfig(conf map[string]Store) {
	log.Info("Starting to reload config")

	encoded, err := json.Marshal(conf)
	if err != nil {
		log.Errorf("Can't marshal config when reloding: %s", err.Error())
	} else {
		err = ioutil.WriteFile("config.json", encoded, 0755)
		if err != nil {
			log.Errorf("Can't save new config.json: %s", err.Error())
		} else {
			log.Info("New config.json file saved")
		}
	}

	loadCommonSettings(conf)
	loadServerSettings(conf)
	validateConfig(conf)
}

func OnUpdate(fn func(map[string]Store)) {
	storeUpdateHandlers = append(storeUpdateHandlers, fn)
}

func updated(config map[string]Store) {
	for _, fn := range storeUpdateHandlers {
		fn(config)
	}
}

func readConfig(confFile string) {
	log.Info("Try to load config from: " + confFile)
	file, err := ioutil.ReadFile(confFile)
	if err != nil {
		log.Errorf("Config file read error: %v", err.Error())
		return
	}

	var conf map[string]Store
	err = json.Unmarshal(file, &conf)
	if err != nil {
		log.Error("Error when read objects config", err.Error())
		time.Sleep(time.Microsecond * 200)
		os.Exit(1)
	}
	loadCommonSettings(conf)
	loadServerSettings(conf)
	validateConfig(conf)
}

func loadCommonSettings(conf map[string]Store) {
	cs, ok := conf[ObjCommonSettings]
	if !ok {
		log.Warn("No common settings in config")
		return
	}
	encoded, err := json.Marshal(cs.Entries)
	if err != nil {
		log.Error("Can't marshal common settings", cs.Entries, err.Error())
	} else {
		err = json.Unmarshal(encoded, CommonSettings)
		if err != nil {
			log.Error("Can't unmarshal common settings", string(encoded), err.Error())
		}
	}
	encoded, err = json.Marshal(cs.I18n)
	if err != nil {
		log.Error("Can't marshal common i18n", cs.I18n, err.Error())
		return
	}
	err = json.Unmarshal(encoded, &CommonSettings.I18n)
	if err != nil {
		log.Error("Can't unmarshal common i18n", string(encoded), err.Error())
	}
}

func loadServerSettings(conf map[string]Store) {
	ss, ok := conf[ObjServerSettings]
	if !ok {
		log.Warn("No server settings in config")
		return
	}
	encoded, err := json.Marshal(ss.Entries)
	if err != nil {
		log.Error("Can't marshal server settings", ss.Entries, err.Error())
		return
	}
	err = json.Unmarshal(encoded, ServerSettings)
	if err != nil {
		log.Error("Can't unmarshal server settings", string(encoded), err.Error())
	}
}

func validateConfig(conf map[string]Store) {
	confLocker.Lock()
	defer confLocker.Unlock()
	_conf := map[string]Store{}
	var err error

	for store, o := range conf {
		log.Info("Parsing config for store:", store)
		o.Store = store
		if o.Props == nil {
			o.Props = map[string]Prop{}
		}
		// if o.HeaderProperty == "" {
		// 	o.HeaderProperty = "name"
		// }

		// Checking object type
		switch o.Type {
		case ObjDirectory:
			//			log.Info("Store is 'directory' type")
		case ObjProcess:
			//			log.Info("Store is 'process' type")
		case ObjMap:
			//			log.Info("Store is 'inConfigSet' type")
			o.Props = nil
		case ObjWorkspace:
			//			log.Info("Store is 'workspace' type")
			o.Props = nil
		case ObjCampaign:
			//			log.Info("Store is 'campaign' type")
		case ObjNotification:
			//			log.Info("Store is 'notification' type")
		case ObjSingle:
			//			log.Info("Store is 'single' type")
		case ObjFile:
			// 		log.Info("Store is 'file' type")
		case ObjProxy:
			// 		log.Info("Store is 'proxy' type")
		default:
			o.Type = ObjDirectory
		}

		allPropsValid := true

		err = o.validateProps(o.Props, true)
		if err != nil {
			log.Error("Validating props failed:", err)
			allPropsValid = false
			continue
		}

		// prepare HtmlFile for props
		if err = o.preparePropHtmlTemplates(); err != nil {
			log.Error("Preparing HTML templates failed:", err)
			allPropsValid = false
			continue
		}

		//compile actions
		if err = o.compileActions(); err != nil {
			log.Error("Compiling actions failed:", err)
			allPropsValid = false
			continue
		}

		//compile hooks
		if err = o.prepareHooks(true); err != nil {
			log.Error("Preparing hooks failed:", err)
			allPropsValid = false
			continue
		}

		//create tasks
		if err = o.createTasks(); err != nil {
			log.Error("Creating tasks failed:", err)
			allPropsValid = false
			continue
		}

		if allPropsValid {
			_conf[store] = o
		} else {
			log.Error("Invalid Store", store, o)
		}
	}

	config = map[string]Store{}
ConfLoop:
	for storeName := range _conf {
		store := _conf[storeName]
		for name, p := range store.Props {
			if p.Type == PropRef || p.Type == PropRefList || p.Type == PropVirtualRefList {
				_, ok := _conf[p.Store]
				if !ok {
					log.Error("Oppostite store '" + p.Store + "' not exists for prop '" + name + "' in store '" + storeName + "'. Store will ignored!")
					continue ConfLoop
				}
			}

			for subName, subP := range p.Props {
				if subP.Type == PropRef || subP.Type == PropRefList || subP.Type == PropVirtualRefList {
					_, ok := _conf[subP.Store]
					if !ok {
						log.Error("Oppostite store '" + subP.Store + "' not exists for prop '" + name + "." + subName + "' in store '" + storeName + "'. Store will ignored!")
						continue ConfLoop
					}
				}
			}
		}

		switch storeName {
		case DefaultDirectory, DefaultSingle, DefaultCampaign, DefaultNotification, DefaultProcess:
			//			log.Info("This is", store, "store")
		default:
			if defaultDirectory, ok := _conf[DefaultDirectory]; ok {
				store.mergeFilters(&defaultDirectory)
				for _pName, _prop := range defaultDirectory.Props {
					store.LoadDefaultIntoProp(_pName, _prop)
				}
			}
			switch store.Type {
			case ObjProcess:
				if defaultProcess, ok := _conf[DefaultProcess]; ok {
					store.mergeFilters(&defaultProcess)
					for _pName, _prop := range defaultProcess.Props {
						store.LoadDefaultIntoProp(_pName, _prop)
					}
				}
			case ObjNotification:
				if defaultNotification, ok := _conf[DefaultNotification]; ok {
					store.mergeAccess(&defaultNotification)
					for _pName, _prop := range defaultNotification.Props {
						store.LoadDefaultIntoProp(_pName, _prop)
					}
				}
			case ObjSingle:
				if defaultSingle, ok := _conf[DefaultSingle]; ok {
					store.mergeFilters(&defaultSingle)
					for _pName, _prop := range defaultSingle.Props {
						store.LoadDefaultIntoProp(_pName, _prop)
					}
				}
			}
		}

		config[store.Store] = store
		if store.HTTPApi {
			HTTPApiEnabledStores = append(HTTPApiEnabledStores, store)
		}
	}

	for storeName, _store := range config {
		if _store.Type == ObjProxy {

			baseStore, ok := config[_store.BaseStore]
			if !ok {
				log.Error("Can't find baseStore " + _store.BaseStore + " for proxy store " + _store.Store)
				delete(config, _store.Store)
				continue
			}
			if baseStore.Proxies == nil {
				baseStore.Proxies = []string{}
			}
			baseStore.Proxies = append(baseStore.Proxies, _store.Store)
			config[baseStore.Store] = baseStore

			// cloning base store
			encoded, _ := json.Marshal(baseStore)
			var store Store
			json.Unmarshal(encoded, &store)

			store.Store = storeName
			store.BaseStore = _store.BaseStore
			store.Type = ObjProxy

			if _store.Access != nil {
				store.Access = _store.Access
			}
			store.Actions = _store.Actions
			if _store.NavOrder != 0 {
				store.NavOrder = _store.NavOrder
			}
			if _store.NavGroup != "" {
				store.NavGroup = _store.NavGroup
			}
			if _store.Display != "" {
				store.Display = _store.Display
			}
			if _store.HeaderTemplate != "" {
				store.HeaderTemplate = _store.HeaderTemplate
			}
			if _store.HeaderProperty != "" {
				store.HeaderProperty = _store.HeaderProperty
			}
			if _store.Filters != nil {
				store.Filters = _store.Filters
			}
			if _store.Labels != nil {
				store.Labels = _store.Labels
			}

			err := DB.Save(bucket, store.Store, store)
			if err != nil {
				log.Error("Error when saving object in conf", err.Error())
			}
			_store = store
		}
	}
	updated(config)
}

func (m *Store) preparePropHtmlTemplates() (err error) {
	return nil
}

func (m *Store) compileActions() (err error) {
	var actionIds = []string{}
	if m.Actions != nil && len(m.Actions) > 0 {
		for i, a := range m.Actions {
			if !actionIdRgx.MatchString(a.ID) {
				return errors.New("Invalid action name. Must start with a letter or underscore and contains only letters, digits or underscores")
			}
			if a.Type == "client" {
				continue
			}
			actionIds = append(actionIds, a.ID)
			if a.Script != "" {
				script := a.Script
				if a.Disabled != nil {
					switch a.Disabled.(type) {
					case string:
						disabled := a.Disabled.(string)
						script = `if (` + disabled + `) {console.error("Action is disabled"); return "Action is disabled"};
						` + script
					case bool:
						disabled := a.Disabled.(bool)
						if disabled {
							script = `console.error("Action is disabled"); return "Action is disabled"`
						}
					default:
						return errors.New("Invalid action " + a.ID + ". Invalid Disabled property")
					}
				}
				if a.Hidden != nil {
					switch a.Hidden.(type) {
					case string:
						hidden := a.Hidden.(string)
						script = `if (` + hidden + `) {console.error("Action is hidden"); return "Action is hidden"};
						` + script
					case bool:
						hidden := a.Hidden.(bool)
						if hidden {
							script = `console.error("Action is hidden"); return "Action is hidden"`
						}
					default:
						return errors.New("Invalid action " + a.ID + ". Invalid hidden property")
					}
				}
			}
			for k, v := range m.Actions[i].Props {
				if v.Type == "" {
					v.Type = PropString
				}
				m.Actions[i].Props[k] = v
			}
			if a.ConcurentCallsLimit > 0 {
				id := m.Store + "actions" + a.ID
				concurrentChannels[id] = make(chan struct{}, a.ConcurentCallsLimit)
			}
		}
	}
	sort.Strings(actionIds)
	if m.StoreActions != nil && len(m.StoreActions) > 0 {
		for _, a := range m.StoreActions {
			if !actionIdRgx.MatchString(a.ID) {
				return errors.New("Invalid action name. Must start with a letter or underscore and contains only letters, digits or underscores")
			}
			if len(actionIds) > 0 && array.IndexOfSortedStrings(actionIds, a.ID) != -1 {
				return errors.New("Can't create store action with _id " + a.ID + " for store " + m.Store + " because action is present with the same _id")
			}
			if a.Script != "" {
				script := a.Script
				if a.Disabled != nil {
					switch a.Disabled.(type) {
					case string:
						disabled := a.Disabled.(string)
						script = `if (` + disabled + `) {console.error("Action is disabled"); return "Action is disabled"};
						` + script
					case bool:
						disabled := a.Disabled.(bool)
						if disabled {
							script = `console.error("Action is disabled"); return "Action is disabled"`
						}
					default:
						return errors.New("Invalid action " + a.ID + ". Invalid disabled property")
					}
				}
				if a.Hidden != nil {
					switch a.Hidden.(type) {
					case string:
						hidden := a.Hidden.(string)
						script = `if (` + hidden + `) {console.error("Action is hidden"); return "Action is hidden"};
						` + script
					case bool:
						hidden := a.Hidden.(bool)
						if hidden {
							script = `console.error("Action is hidden"); return "Action is hidden"`
						}
					default:
						return errors.New("Invalid action " + a.ID + ". Invalid hidden property")
					}
				}
				if a.ConcurentCallsLimit > 0 {
					id := m.Store + "actions" + a.ID
					concurrentChannels[id] = make(chan struct{}, a.ConcurentCallsLimit)
				}
			}
		}
	}
	return nil
}

func (m *Store) prepareHooks(compile bool) (err error) {

	return nil
}

func (m *Store) createTasks() error {
	// for i, t := range m.Tasks {
	// }
	return nil
}

func (m *Store) validateProps(props map[string]Prop, parseObjects bool) error {
	for pName, prop := range props {
		prop.Name = pName
		// Processing Type
		if prop.Type == "" {
			prop.Type = PropString
		}
		switch prop.Type {
		case PropWidget, PropAction, PropFile, PropFileList, PropPassword:
			continue
		case PropAny:
			continue
		case PropInt:
			prop.clearStringParams()
			prop.clearRefParams()
			prop.clearObjectParams()
			if prop.Default != nil {
				if d, ok := prop.Default.(map[string]interface{}); ok {
					if d["$expression"] == nil {
						return errors.New("Invalid default int value in prop: '" + pName + "'")
					}
				} else if _, ok := prop.checkDefaultInt(); !ok {
					return errors.New("Invalid default int value in prop: '" + pName + "'")
				}
			}
			_, _, ok := prop.checkMinMaxParams()
			if !ok {
				return errors.New("Wrong min-max params in prop: '" + pName + "'")
			}
			//			if prop.Values != nil && len(prop.Values) > 0 {
			//				for _, v := range prop.Values {
			//					if _, ok := v.Value.(float64); !ok {
			//						return errors.New("Invalid int value in list in prop: '" + pName + "'")
			//					}
			//				}
			//			}
			props[pName] = prop
		case PropFloat:
			prop.clearStringParams()
			prop.clearRefParams()
			prop.clearObjectParams()
			if prop.Default != nil {
				if d, ok := prop.Default.(map[string]interface{}); ok {
					if d["$expression"] == nil {
						return errors.New("Invalid default float value in prop: '" + pName + "'")
					}
				} else if _, ok := prop.checkDefaultFloat(); !ok {
					return errors.New("Invalid default float value in prop: '" + pName + "'")
				}
			}
			_, _, ok := prop.checkMinMaxParams()
			if !ok {
				return errors.New("Wrong min-max params in prop: '" + pName + "'")
			}
			props[pName] = prop
		case PropBool:
			prop.clearStringParams()
			prop.clearRefParams()
			prop.clearNumberParams()
			prop.clearObjectParams()
			if prop.Default != nil {
				if d, ok := prop.Default.(map[string]interface{}); ok {
					if d["$expression"] == nil {
						return errors.New("Invalid default bool value in prop: '" + pName + "'")
					}
				} else if _, ok := prop.Default.(bool); !ok {
					return errors.New("Invalid default bool value in prop: '" + pName + "'")
				}
			}
			props[pName] = prop
		case PropString:
			prop.clearNumberParams()
			prop.clearRefParams()
			prop.clearObjectParams()
			if prop.Default != nil {
				if d, ok := prop.Default.(map[string]interface{}); ok {
					if d["$expression"] == nil {
						return errors.New("Invalid default string value in prop: '" + pName + "'")
					}
				} else if _, ok := prop.Default.(string); !ok {
					return errors.New("Invalid default string value in prop: '" + pName + "'")
				}
			}
			if prop.MinLength < 0 || prop.MaxLength < 0 {
				return errors.New("Wrong minLength or maxLength values in prop: '" + pName + "'")
			}
			if prop.MinLength != 0 && prop.MaxLength != 0 {
				if prop.MinLength > prop.MaxLength {
					return errors.New("minLength > maxLength in prop: '" + pName + "'")
				}
			}
			props[pName] = prop
		case PropDate:
			prop.clearStringParams()
			prop.clearRefParams()
			prop.clearObjectParams()
			if prop.Default != nil {
				if d, ok := prop.Default.(map[string]interface{}); ok {
					if d["$expression"] == nil {
						return errors.New("Invalid default date value in prop: '" + pName + "'")
					}
				} else if _, ok := prop.Default.(time.Time); !ok {
					return errors.New("Invalid default date in prop: '" + pName + "'")
				}
			}
			props[pName] = prop
		case PropRef:
			prop.clearStringParams()
			prop.clearNumberParams()
			prop.clearObjectParams()
			if prop.Default != nil {
				if d, ok := prop.Default.(map[string]interface{}); ok {
					if d["$expression"] == nil {
						return errors.New("Invalid default ref value in prop: '" + pName + "'")
					}
				} else if _, ok := prop.Default.(string); !ok {
					return errors.New("Invalid default value for ref type in prop: '" + pName + "'")
				}
			}
			prop.Default = nil
			if prop.Store == "" {
				return errors.New("Store not provided for ref type in prop: '" + pName + "'")
			}
			props[pName] = prop
		case PropRefList:
			prop.clearStringParams()
			prop.clearNumberParams()
			prop.clearObjectParams()
			if prop.Default != nil {
				if d, ok := prop.Default.(map[string]interface{}); ok {
					if d["$expression"] == nil {
						return errors.New("Invalid default refList value in prop: '" + pName + "'")
					}
				} else if _, ok := prop.Default.([]interface{}); !ok {
					return errors.New("Invalid default value for refList type in prop: '" + pName + "'")
				}
			}
			if prop.Store == "" {
				return errors.New("Store not provided for refList type in prop: '" + pName + "'")
			}
			props[pName] = prop
		case PropVirtual:
			prop.clearStringParams()
			prop.clearRefParams()
			prop.clearNumberParams()
			prop.clearObjectParams()
			prop.Default = nil
			props[pName] = prop
		case PropObject:
			if !parseObjects {
				return errors.New("Recursive objects not allowed '" + pName + "'")
			}
			prop.clearStringParams()
			prop.clearRefParams()
			prop.clearNumberParams()
			//			prop.Default = nil
			err := m.validateProps(prop.Props, false)
			if err != nil {
				return err
			}
			props[pName] = prop
		case PropObjectList:
			if !parseObjects {
				return errors.New("Recursive objects not allowed '" + pName + "'")
			}
			prop.Pattern = ""
			prop.Mask = ""
			prop.clearRefParams()
			prop.clearNumberParams()
			err := m.validateProps(prop.Props, false)
			if err != nil {
				return err
			}
			props[pName] = prop
		case PropVirtualRefList:
			prop.clearStringParams()
			prop.clearNumberParams()
			prop.clearObjectParams()
			prop.Default = nil
			if prop.Store == "" {
				return errors.New("Store is not provided for virtualRefList type in prop: '" + pName + "'")
			}
			if prop.ForeignKey == "" && prop.Query == nil {
				return errors.New("Foregn key or query is not provided for virtualRefList type in prop: '" + pName + "'")
			}
			props[pName] = prop
		case PropComments:
			prop.clearStringParams()
			prop.clearNumberParams()
			props[pName] = prop
		case PropVirtualClient:
		default:
			return errors.New("Unknown prop type: '" + pName + "' '" + prop.Type + "'")
		}
	}
	return nil
}

func (p *Prop) checkDefaultFloat() (float64, bool) {
	_def, ok := p.Default.(float64)
	if !ok {
		return 0, false
	}
	return _def, true
}

func (p *Prop) checkDefaultInt() (int, bool) {
	_def, ok := p.checkDefaultFloat()
	if !ok {
		return 0, false
	}
	def := int(_def)
	return def, true
}

func (p *Prop) checkMinMaxParams() (float64, float64, bool) {
	var min, max float64
	if p.Min != nil {
		var ok bool
		min, ok = p.Min.(float64)
		if !ok {
			return 0, 0, false
		}
	}
	if p.Max != nil {
		var ok bool
		max, ok = p.Max.(float64)
		if !ok {
			return 0, 0, false
		}
	}
	if min == 0 && max == 0 {
		return min, max, true
	}
	if min > max {
		return 0, 0, false
	}
	return min, max, true
}

func (p *Prop) clearNumberParams() {
	p.Min = nil
	p.Max = nil
}

func (p *Prop) clearObjectParams() {
	p.Props = nil
}

func (p *Prop) clearRefParams() {
	p.Store = ""
	p.PopulateIn = ""
	p.OppositeProp = ""
	p.ExtraQuery = nil
	p.Query = nil
}

func (p *Prop) clearStringParams() {
	p.MinLength = 0
	p.MaxLength = 0
	p.Pattern = ""
	p.Mask = ""
}

func (m *Store) LoadDefaultIntoProp(name string, p Prop) {
	if m.Props == nil {
		m.Props = map[string]Prop{}
	}
	if !p.Configurable {
		m.Props[name] = p
		return
	}

	prop, ok := m.Props[name]
	if !ok {
		m.Props[name] = p
		return
	}

	if prop.Type != "" {
		p.Type = prop.Type
	}
	if prop.FormGroup != "" {
		p.FormGroup = prop.FormGroup
	}
	if prop.FormTab != "" {
		p.FormTab = prop.FormTab
	}
	if prop.FormOrder != 0 {
		p.FormOrder = prop.FormOrder
	}
	if prop.Access != nil {
		p.Access = prop.Access
	}
	if prop.Display != "" {
		p.Display = prop.Display
	}
	// TODO придумать как поступать с булевыми полями. Если оно отсутствует в JSON, то всегда будет false
	p.ReadOnly = prop.ReadOnly
	p.Required = prop.Required

	if prop.Default != nil {
		p.Default = prop.Default
	}
	if prop.MinLength != 0 {
		p.MinLength = prop.MinLength
	}
	if prop.MaxLength != 0 {
		p.MaxLength = prop.MaxLength
	}
	if prop.Min != nil {
		p.Min = prop.Min
	}
	if prop.Max != nil {
		p.Max = prop.Max
	}
	if prop.Hidden != nil {
		p.Hidden = prop.Hidden
	}
	if prop.Pattern != nil {
		p.Pattern = prop.Pattern
	}
	if prop.Mask != nil {
		p.Mask = prop.Mask
	}
	if prop.Load != "" {
		p.Load = prop.Load
	}
	if prop.Store != "" {
		p.Store = prop.Store
	}
	if prop.PopulateIn != "" {
		p.PopulateIn = prop.PopulateIn
	}
	if prop.Label != "" {
		p.Label = prop.Label
	}
	if prop.Placeholder != "" {
		p.Placeholder = prop.Placeholder
	}
	if prop.Disabled != "" {
		p.Disabled = prop.Disabled
	}
	if len(prop.SearchBy) != 0 {
		p.SearchBy = prop.SearchBy
	}
	if prop.OppositeProp != "" {
		p.OppositeProp = prop.OppositeProp
	}

	m.Props[name] = p
}

func (m *Store) mergeAccess(defaultStore *Store) {
	if len(m.Access) == 0 && len(defaultStore.Access) > 0 {
		for i := range defaultStore.Access {
			m.Access = append(m.Access, defaultStore.Access[i])
		}
	}
}

func (m *Store) mergeFilters(defaultStore *Store) {
	if len(defaultStore.Filters) == 0 {
		return
	}
	if len(m.Filters) == 0 {
		m.Filters = map[string]Filter{}
	}
	for k, v := range defaultStore.Filters {
		f, ok := m.Filters[k]
		if !ok {
			m.Filters[k] = v
			continue
		}
		if f.Label == "" {
			f.Label = v.Label
		}
		if f.Display == "" {
			f.Display = v.Display
		}
		if f.Placeholder == "" {
			f.Placeholder = v.Placeholder
		}
		if len(f.Conditions) == 0 {
			f.Conditions = v.Conditions
		}
		if len(f.SearchBy) == 0 {
			f.SearchBy = v.SearchBy
		}
		if f.Store == "" {
			f.Store = v.Store
		}
		if f.FilterBy == "" {
			f.FilterBy = v.FilterBy
		}
		if len(f.Options) == 0 {
			f.Options = v.Options
		}
		if f.Mask == "" {
			f.Mask = v.Mask
		}
		if !f.Multi {
			f.Multi = v.Multi
		}
		m.Filters[k] = f
	}
}

func mergeModels(from, to *Store) {
	if from.Filters != nil {
		to.Filters = from.Filters
	}
	if from.NavGroup != "" {
		to.NavGroup = from.NavGroup
	}
	if from.FormGroups != nil {
		to.FormGroups = from.FormGroups
	}
	if len(from.I18n) > 0 {
		mergo.MergeWithOverwrite(&to.I18n, from.I18n)
	}
	if len(from.Entries) > 0 {
		mergo.MergeWithOverwrite(&to.Entries, from.Entries)
	}
	if from.NavOrder != 0 {
		to.NavOrder = from.NavOrder
	}
	if from.Display != "" {
		to.Display = from.Display
	}
	if from.Icon != "" {
		to.Icon = from.Icon
	}
	if from.PrepareItemsScript != "" {
		to.PrepareItemsScript = from.PrepareItemsScript
	}
	if len(from.Labels) > 0 {
		to.Labels = from.Labels
	}
	if len(from.TableColumns) > 0 {
		to.TableColumns = from.TableColumns
	}
	if from.OrderBy != "" {
		to.OrderBy = from.OrderBy
	}
	if from.HTML != "" {
		to.HTML = from.HTML
	}
	if from.Label != "" {
		to.Label = from.Label
	}
	if from.NavLabel != "" {
		to.NavLabel = from.NavLabel
	}
	if from.Template != "" {
		to.Template = from.Template
	}
	if from.TemplateFile != "" {
		to.TemplateFile = from.TemplateFile
	}
	if from.ListViewOnly != false {
		to.ListViewOnly = from.ListViewOnly
	}
	if len(from.TableColumns) > 0 {
		to.TableColumns = from.TableColumns
	}
	if len(from.Actions) > 0 {
	FromActionsLoop:
		for _, v := range from.Actions {
			for tk, tv := range to.Actions {
				if v.ID == tv.ID {
					if v.Label != "" {
						tv.Label = v.Label
					}
					to.Actions[tk] = tv
					continue FromActionsLoop
				}
			}
			to.Actions = append(to.Actions, v)
		}
	}
	for k, vFrom := range from.Props {
		if vTo, ok := to.Props[k]; ok {
			mergeProps(&vFrom, &vTo)
			to.Props[k] = vTo
			continue
		}
		if to.Store == "_profile" {
			switch vFrom.Type {
			case PropVirtual, PropVirtualClient, PropAction:
				to.Props[k] = vFrom
			}
		}
	}
}

func mergeProps(from, to *Prop) {
	if from.TableLink {
		to.TableLink = from.TableLink
	}
	if from.FormGroup != "" {
		to.FormGroup = from.FormGroup
	}
	if from.FormOrder != 0 {
		to.FormOrder = from.FormOrder
	}
	if from.Display != "" {
		to.Display = from.Display
	}
	if from.HTML != "" {
		to.HTML = from.HTML
	}
	if from.DisplayWidth != 0 {
		to.DisplayWidth = from.DisplayWidth
	}
	if from.Style != nil {
		to.Style = from.Style
	}
	if from.Default != nil {
		to.Default = from.Default
	}
	if from.Pattern != "" { //TODO: Must be compile
		to.Pattern = from.Pattern
	}
	if from.Mask != "" {
		to.Mask = from.Mask
	}
	if from.Placeholder != "" {
		to.Placeholder = from.Placeholder
	}
	if from.MaxLength != 0 {
		to.MaxLength = from.MaxLength
	}
	if from.MinLength != 0 {
		to.MinLength = from.MinLength
	}
	if len(from.Options) > 0 {
		to.Options = from.Options
	}
	if from.Hidden != nil {
		to.Hidden = from.Hidden
	}
	if from.Disabled != nil {
		to.Disabled = from.Disabled
	}
	if from.Required != nil {
		to.Required = from.Required
	}
	if from.FormTab != "" {
		to.FormTab = from.FormTab
	}
	for k, vFrom := range from.Props {
		if vTo, ok := to.Props[k]; ok {
			mergeProps(&vFrom, &vTo)
			to.Props[k] = vTo
		}
	}
}
